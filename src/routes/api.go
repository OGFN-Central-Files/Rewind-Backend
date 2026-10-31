package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"go-backend/src/database/models"
	"go-backend/src/discord"
	"go-backend/src/discord/InteractionMessageComponents"
	"go-backend/src/global"
	"go-backend/src/services"
	BattlePassComponent "go-backend/src/services/battlepass"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
	"go.mongodb.org/mongo-driver/bson"
)

func InitApi() {
	http.HandleFunc("/api/v1/health", HealthCheckHandler)
	http.HandleFunc("/fortnite/api/storefront/v2/keychain", HandleKeychain)
	http.HandleFunc("/fortnite/api/game/v2/chat/{f}/{ff}/{fff}/pc", ReturnEmptyJson)
	http.HandleFunc("/fortnite/api/game/v2/chat/{f}/recommendGeneralChatRooms/pc", ReturnEmptyJson)

	http.HandleFunc("/api/v2/rewind/clip", handleClipComp)
	http.HandleFunc("/api/v2/rewind/auth", handleFirstLogin)
	http.HandleFunc("/api/v2/rewind/discord", handleDiscordOAuth)
	http.HandleFunc("/api/v2/rewind/discord/home", handleHomepageReq)
	http.HandleFunc("/api/v2/rewind/sessions", HandleSessionFetch)
	http.HandleFunc("/api/v2/rewind/launcher/decode", handleLauncherDecodeRequest)
	http.HandleFunc("/api/v2/rewind/launcher/ban_check", handleRetrieveBanStatusRequest)
	http.HandleFunc("/api/v2/rewind/launcher/version_check", handleVersionCheck)
	http.HandleFunc("/api/v2/rewind/launcher/events", handleEventsRequest)
	http.HandleFunc("/api/v2/rewind/launcher/status", handleStatusRequest)
	http.HandleFunc("/api/v2/rewind/launcher/builds", handleBuildsRequest)
	http.HandleFunc("/api/v2/rewind/launcher/stats", HandleLauncherStatFetch)
	http.HandleFunc("/api/v2/rewind/launcher/leaderboard", HandleLeaderboard)

	http.HandleFunc("/api/v2/dedicated/stats", HandleSystemStats)
	http.HandleFunc("/api/v2/dedicated/profile_update", HandleDedicatedDataUpload)
	http.HandleFunc("/api/v2/dedicated/verify", HandleDedicatedVerify)

	http.HandleFunc("/auth/v2/account/validate", ValidateXMPP)
}

var (
	keychainData      []byte
	keychainDataMutex sync.RWMutex
	lastFetchedTime   time.Time
	cacheDuration     = 1 * time.Hour
)

type LeaderboardEntry struct {
	Username                  string `json:"username"`
	Kills                     int    `json:"kills"`
	Wins                      int    `json:"wins"`
	Hype                      int    `json:"hype"`
	KillsLeaderboardPlacement int    `json:"killsLeaderboardPlacement"`
	WinsLeaderboardPlacement  int    `json:"winsLeaderboardPlacement"`
	HypeLeaderboardPlacement  int    `json:"hypeLeaderboardPlacement"`
}

type SystemStats struct {
	CPUUsage float64 `json:"cpu_usage_percent"`
	RAMUsage float64 `json:"ram_usage_percent"`
	Uptime   string  `json:"uptime"`
}

const (
	clientID     = "1380659212547260619"
	clientSecret = "KBBkrdNuwHrbxG9xFy-aOiSTAPV00cav"
	redirectURI  = "https://backend-services-prod.privateuser.xyz/api/v2/rewind/discord"
)

func ReturnEmptyJson(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{})
}

func handleClipComp(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	code := query.Get("code")

	if code == "" {
		http.Redirect(w, r, generateOAuthURL("ClipComp"), http.StatusFound)
		return
	}

	token, err := getDiscordToken(code, "ClipComp")
	if err != nil {
		http.Error(w, "Failed to get access token", http.StatusInternalServerError)
		return
	}

	discordID, err := getDiscordId(token)
	if err != nil {
		http.Error(w, "Failed to fetch user info", http.StatusInternalServerError)
		return
	}

	avatarInfo, err := getUserAvatar(token)
	if err != nil {
		http.Error(w, "Failed to fetch user avatar", http.StatusInternalServerError)
		return
	}

	memberRole, err := getMemberRoles(discord.BotToken, "1380268084237045880", discordID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var avatar_url string
	avatarData, ok := avatarInfo["avatar"].(string)
	if !ok {
		avatar_url = "https://ia600305.us.archive.org/31/items/discordprofilepictures/discordblue.png"
	} else {
		avatar_url = "https://cdn.discordapp.com/avatars/" + avatarInfo["id"].(string) + "/" + avatarData
		if strings.HasPrefix(avatarData, "a_") {
			avatar_url += ".gif"
		} else {
			avatar_url += ".png"
		}
	}

	var existingAccount models.Account
	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	accountsCollection.FindOne(context.Background(), bson.M{"discordid": discordID}).Decode(&existingAccount)

	htmlBytes, err := os.ReadFile("./frontend/home/clipcomp.html")
	if err != nil {
		http.Error(w, "Failed to load HTML file", http.StatusInternalServerError)
		return
	}

	profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")
	var userProfile models.Profile
	_ = profilesCollection.FindOne(context.Background(), bson.M{"accountid": existingAccount.AccountID}).Decode(&userProfile)

	favSkin, found := funcs.GetFavoriteSkin(&userProfile)
	if !found || favSkin == "" {
		favSkin = getRandomSkinID()
	}

	launcherToken, ok := existingAccount.Additional["launcherToken"].(string)
	if !ok || launcherToken == "" {
		launcherToken = "Bearer " + funcs.MakeID()
		existingAccount.Additional["launcherToken"] = launcherToken
		global.LauncherTokens[launcherToken] = map[string]interface{}{
			"launcherToken": launcherToken,
			"accountId":     existingAccount.AccountID,
			"username":      existingAccount.Username,
			"email":         existingAccount.Email,
			"password":      existingAccount.Password,
			"avatar_url":    avatar_url,
			"favoriteSkin":  favSkin,
			"roles":         memberRole,
			"creation_date": time.Now().UTC(),
		}
	}

	existingAccount.Additional["access_token"] = token
	accountsCollection.UpdateOne(
		context.Background(),
		bson.M{"accountid": existingAccount.AccountID},
		bson.M{"$set": bson.M{"additional": existingAccount.Additional}},
	)

	http.SetCookie(w, &http.Cookie{
		Name:     "LauncherToken",
		Value:    launcherToken,
		Path:     "/",
		HttpOnly: false,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	htmlContent := string(htmlBytes)
	updatedHTML := htmlContent

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, updatedHTML)
}

func HandleSystemStats(w http.ResponseWriter, r *http.Request) {
	cpuPercents, err := cpu.Percent(time.Second, false)
	if err != nil || len(cpuPercents) == 0 {
		http.Error(w, "Failed to get CPU usage", http.StatusInternalServerError)
		return
	}

	vmem, err := mem.VirtualMemory()
	if err != nil {
		http.Error(w, "Failed to get RAM usage", http.StatusInternalServerError)
		return
	}

	uptimeSec, err := host.Uptime()
	if err != nil {
		http.Error(w, "Failed to get uptime", http.StatusInternalServerError)
		return
	}

	uptime := time.Duration(uptimeSec) * time.Second

	stats := SystemStats{
		CPUUsage: cpuPercents[0],
		RAMUsage: vmem.UsedPercent,
		Uptime:   uptime.String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func HandleLeaderboard(w http.ResponseWriter, r *http.Request) {
	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")

	filter := bson.M{
		"banned": false,
		"$or": []bson.M{
			{"additional.kills": bson.M{"$gt": 1}},
			{"additional.wins": bson.M{"$gt": 1}},
			{"additional.hype": bson.M{"$gt": 1}},
		},
	}

	cursor, err := accountsCollection.Find(context.Background(), filter)
	if err != nil {
		funcs.CreateErrorV2(
			"errors.com.epicgames.common.bad_request",
			"Internal Server Error",
			[]string{}, -1, "", 401, w,
		)
		return
	}
	defer cursor.Close(context.Background())

	var results []LeaderboardEntry
	for cursor.Next(context.Background()) {
		var account models.Account
		if err := cursor.Decode(&account); err != nil {
			funcs.CreateErrorV2(
				"errors.com.epicgames.common.bad_request",
				"Internal Server Error",
				[]string{}, -1, "", 401, w,
			)
			return
		}

		add := account.Additional
		entry := LeaderboardEntry{
			Username: account.Username,
			Kills:    funcs.IntValue(add["kills"]),
			Wins:     funcs.IntValue(add["wins"]),
			Hype:     funcs.IntValue(add["hype"]),
		}
		results = append(results, entry)
	}

	if err := cursor.Err(); err != nil {
		funcs.CreateErrorV2(
			"errors.com.epicgames.common.bad_request",
			"Internal Server Error",
			[]string{}, -1, "", 401, w,
		)
		return
	}

	assignPlacements(&results, func(a, b LeaderboardEntry) bool {
		return a.Kills > b.Kills
	}, func(e *LeaderboardEntry, i int) {
		e.KillsLeaderboardPlacement = i + 1
	})

	assignPlacements(&results, func(a, b LeaderboardEntry) bool {
		return a.Wins > b.Wins
	}, func(e *LeaderboardEntry, i int) {
		e.WinsLeaderboardPlacement = i + 1
	})

	assignPlacements(&results, func(a, b LeaderboardEntry) bool {
		return a.Hype > b.Hype
	}, func(e *LeaderboardEntry, i int) {
		e.HypeLeaderboardPlacement = i + 1
	})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		funcs.CreateErrorV2(
			"errors.com.epicgames.common.bad_request",
			"Internal Server Error",
			[]string{}, -1, "", 401, w,
		)
		return
	}
}

func assignPlacements(entries *[]LeaderboardEntry, compare func(a, b LeaderboardEntry) bool, setPlacement func(e *LeaderboardEntry, i int)) {
	sorted := make([]LeaderboardEntry, len(*entries))
	copy(sorted, *entries)

	sort.SliceStable(sorted, func(i, j int) bool {
		return compare(sorted[i], sorted[j])
	})

	for i, sortedEntry := range sorted {
		for j := range *entries {
			if (*entries)[j].Username == sortedEntry.Username {
				setPlacement(&(*entries)[j], i)
				break
			}
		}
	}
}

func HandleLauncherStatFetch(w http.ResponseWriter, r *http.Request) {
	count := 0
	now := time.Now()
	twoMinutesAgo := now.Add(-2 * time.Minute)

	playerCount := global.PlayerCount
	if playerCount < 0 {
		playerCount = 0
	}

	sessions := global.Sessions
	if sessions == nil {
		response := map[string]interface{}{
			"player_count": playerCount,
			"server_count": 0,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	for _, session := range sessions {
		parsedTime, err := time.Parse(time.RFC3339, session.LastUpdated)
		if err != nil {
			continue
		}
		if !parsedTime.Before(twoMinutesAgo) {
			count++
		}
	}

	response := map[string]interface{}{
		"player_count": playerCount,
		"server_count": count,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func HandleSessionFetch(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	twoMinutesAgo := now.Add(-2 * time.Minute)

	type ActiveSession struct {
		Started       bool     `json:"started"`
		OwnerId       string   `json:"ownerId"`
		PublicPlayers []string `json:"publicPlayers"`
		SessionId     string   `json:"sessionId"`
		SessionName   string   `json:"sessionName"`
	}

	ip := funcs.GetRealIP(r)
	isVpn, err := funcs.IsVPN(ip)
	if err != nil {
		http.Error(w, "Error checking VPN status: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if isVpn {
		htmlBytes, err := ioutil.ReadFile("./frontend/home/vpn.html")
		if err != nil {
			http.Error(w, "Failed to load HTML file", http.StatusInternalServerError)
			return
		}

		htmlContent := string(htmlBytes)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, htmlContent)

		return
	}

	var activeSessions []ActiveSession

	for _, session := range global.Sessions {
		parsedTime, err := time.Parse(time.RFC3339, session.LastUpdated)
		if err != nil {
			log.Printf("Error parsing LastUpdated for session %s: %v\n", session.SessionId, err)
			continue
		}

		if parsedTime.Before(twoMinutesAgo) || session.Started {
			continue
		}

		name := "unknown"
		if session.OwnerId == "arena" {
			name = "Lategame Arena - EU"
		}

		if session.OwnerId == "solo" {
			name = "Lategame - EU"
		}

		if session.OwnerId == "fullmap" {
			name = "Solo - EU"
		}

		activeSessions = append(activeSessions, ActiveSession{
			Started:       session.Started,
			OwnerId:       session.OwnerId,
			PublicPlayers: session.PublicPlayers,
			SessionId:     session.SessionId,
			SessionName:   name,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(activeSessions); err != nil {
		log.Println("Error encoding JSON response:", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func HandleDedicatedVerify(w http.ResponseWriter, r *http.Request) {
	tokenManager := services.NewTokenManager("JWT_TOKEN_1231289SNN")

	var requestData struct {
		AuthTicket string `json:"auth_ticket"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		log.Printf("Error decoding request body: %v\n", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if requestData.AuthTicket == "" {
		log.Println("Missing or invalid auth_ticket or kill data")
		http.Error(w, "Missing or invalid auth_ticket or kill data", http.StatusBadRequest)
		return
	}

	user := tokenManager.VerifyTokenBeryllium(w, r, requestData.AuthTicket)
	if user == nil {
		log.Println("Unauthorized access attempt")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// auth, ok := global.BerylliumClients[user.AccountID]
	// if !ok || auth.(map[string]interface{})["bAuthenticated"] != true {
	// 	if global.MatchmakingSettings["bMustHaveBerylliumConnected"] == true {
	// 		funcs.CreateErrorV2(
	// 			"errors.com.epicgames.common.unauthorized",
	// 			"User is not authenticated with Beryllium.",
	// 			[]string{}, -1, "", 401, w,
	// 		)
	// 		return
	// 	}
	// }

	if global.TestingSettings["bTestingPreviligedOnly"].(bool) == true && user.Password != global.TestingSettings["GuildBypassPasswords"].(string) {
		additionalData := user.Additional
		if value, ok := additionalData["bTestingPriviliged"].(bool); !ok || !value {
			funcs.CreateErrorV2(
				"errors.com.epicgames.account.account_not_active",
				"You have been logged out from Fortnite.",
				[]string{}, -1, "", 400, w,
			)
			return
		}
	}

	yes, hasCrownExistent := user.Additional["bHasCrown"].(bool)
	if hasCrownExistent {
		if yes {
			w.WriteHeader(http.StatusCreated)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func HandleDedicatedDataUpload(w http.ResponseWriter, r *http.Request) {
	tokenManager := services.NewTokenManager("JWT_TOKEN_1231289SNN")

	var requestData struct {
		AuthTicket string `json:"auth_ticket"`
		Kills      int    `json:"kills"`
		Win        bool   `json:"victory"`
		Xp         int    `json:"xp"`
		Hype       int    `json:"hype"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		log.Printf("Error decoding request body: %v\n", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if requestData.AuthTicket == "" || requestData.Kills < 0 {
		log.Println("Missing or invalid auth_ticket or kill data")
		http.Error(w, "Missing or invalid auth_ticket or kill data", http.StatusBadRequest)
		return
	}

	user := tokenManager.VerifyTokenBeryllium(w, r, requestData.AuthTicket)
	if user == nil {
		log.Println("Unauthorized access attempt")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vbucksPerKillRaw, ok := global.DedicatedSettings["VBucksPerKill"]
	if !ok {
		log.Println("VBucksPerKill not configured")
		http.Error(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	var vbucksPerKill float64

	switch v := vbucksPerKillRaw.(type) {
	case float64:
		vbucksPerKill = v
	case int:
		vbucksPerKill = float64(v)
	default:
		log.Println("VBucksPerKill has unknown type")
		http.Error(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	vbucksToReward := vbucksPerKill * float64(requestData.Kills)

	if requestData.Win {
		vbucksToReward += 250
	}

	hype := int32(requestData.Hype)
	bIsHypeEmpty := false
	if hype == 0 {
		bIsHypeEmpty = true
	}

	type ArenaDivision struct {
		Name               string
		ThresholdToAdvance float64
	}

	var arenaDivisions = []ArenaDivision{
		{Name: "NormalArenaDiv1", ThresholdToAdvance: 400},
		{Name: "NormalArenaDiv2", ThresholdToAdvance: 800},
		{Name: "NormalArenaDiv3", ThresholdToAdvance: 1500},
		{Name: "NormalArenaDiv4", ThresholdToAdvance: 2000},
		{Name: "NormalArenaDiv5", ThresholdToAdvance: 3500},
		{Name: "NormalArenaDiv6", ThresholdToAdvance: 4500},
		{Name: "NormalArenaDiv7", ThresholdToAdvance: 7500},
		{Name: "NormalArenaDiv8", ThresholdToAdvance: 9000},
		{Name: "NormalArenaDiv9", ThresholdToAdvance: 12000},
		{Name: "NormalArenaDiv10", ThresholdToAdvance: 20000},
	}

	additional := user.Additional
	if !bIsHypeEmpty {

		div, ok := additional["div"].(int32)
		if !ok {
			div = 1
		}
		divInt := int(div)

		hypeVal, ok := additional["hype"].(int32)
		if !ok {
			hypeVal = 0
		}

		hypeVal += hype
		additional["div"] = int32(divInt)
		additional["hype"] = hypeVal

		if divInt >= 1 && divInt <= 10 {
			currentDivision := arenaDivisions[divInt-1]
			if float64(hypeVal) >= currentDivision.ThresholdToAdvance {
				if divInt < 10 {
					divInt += 1
					additional["div"] = int32(divInt)
				}
			}
		}
	}

	kills, ok := additional["kills"].(int32)
	if !ok {
		kills = 0
	}

	wins, ok := additional["wins"].(int32)
	if !ok {
		wins = 0
	}

	kills = int32(kills) + int32(requestData.Kills)
	additional["kills"] = int32(kills)

	if requestData.Win {
		wins += 1
	}

	bHasCrown, isValid := additional["bHasCrown"].(bool)
	if isValid && bHasCrown && requestData.Win {
		profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")
		var existingProfile *models.Profile
		err := profilesCollection.FindOne(context.Background(), bson.M{"accountid": user.AccountID}).Decode(&existingProfile)
		if err == nil {
			profileId := "athena"
			profileData, ok := existingProfile.ProfileData[profileId].(map[string]interface{})
			if !ok {
				profileData = map[string]interface{}{
					"items":           make(map[string]interface{}),
					"stats":           map[string]interface{}{"attributes": make(map[string]interface{})},
					"rvn":             0,
					"commandRevision": 0,
					"updated":         time.Now().UTC().Format(time.RFC3339),
				}
				existingProfile.ProfileData[profileId] = profileData
			}

			items, ok := profileData["items"].(map[string]interface{})
			if !ok {
				items = make(map[string]interface{})
				profileData["items"] = items
			}

			itemId := "VictoryCrown:defaultvictorycrown"
			if _, exists := items[itemId]; !exists {
				items[itemId] = map[string]interface{}{
					"templateId":  itemId,
					"purchasedAt": time.Now().UTC().Format(time.RFC3339),
					"attributes": map[string]interface{}{
						"item_seen": false,
						"variants":  []interface{}{},
						"victory_crown_account_data": map[string]interface{}{
							"has_victory_crown":                   false,
							"data_is_valid_for_mcp":               true,
							"total_victory_crowns_bestowed_count": 1,
							"total_royal_royales_achieved_count":  1,
						},
					},
					"quantity": 1,
				}

				items["AthenaDance:EID_Coronet"] = map[string]interface{}{
					"templateId":  "AthenaDance:EID_Coronet",
					"purchasedAt": time.Now().UTC().Format(time.RFC3339),
					"attributes": map[string]interface{}{
						"item_seen": false,
						"variants":  []interface{}{},
					},
					"quantity": 1,
				}

				_, err = profilesCollection.UpdateOne(context.Background(), bson.M{"accountid": user.AccountID}, bson.M{
					"$set": bson.M{"profiledata.athena.items": items},
				})
			}
		}
	}

	additional["wins"] = int32(wins)
	additional["bHasCrown"] = false

	if requestData.Win {
		additional["bHasCrown"] = true

		profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")
		var existingProfile *models.Profile
		err := profilesCollection.FindOne(context.Background(), bson.M{"accountid": user.AccountID}).Decode(&existingProfile)
		if err == nil {
			profileId := "athena"
			profileData, ok := existingProfile.ProfileData[profileId].(map[string]interface{})
			if !ok {
				profileData = map[string]interface{}{
					"items":           make(map[string]interface{}),
					"stats":           map[string]interface{}{"attributes": make(map[string]interface{})},
					"rvn":             0,
					"commandRevision": 0,
					"updated":         time.Now().UTC().Format(time.RFC3339),
				}
				existingProfile.ProfileData[profileId] = profileData
			}

			items, ok := profileData["items"].(map[string]interface{})
			if !ok {
				items = make(map[string]interface{})
				profileData["items"] = items
			}

			itemId := "VictoryCrown:defaultvictorycrown"
			item, ok := items[itemId].(map[string]interface{})
			if ok {
				if attributes, ok := item["attributes"].(map[string]interface{}); ok {
					if crownData, ok := attributes["victory_crown_account_data"].(map[string]interface{}); ok {
						crownData["has_victory_crown"] = additional["bHasCrown"].(bool)
						crownData["total_victory_crowns_bestowed_count"] = funcs.ToInt(crownData["total_victory_crowns_bestowed_count"]) + 1
						crownData["total_royal_royales_achieved_count"] = funcs.ToInt(crownData["total_royal_royales_achieved_count"]) + 1
					}
				}
			}

			_, err = profilesCollection.UpdateOne(context.Background(), bson.M{"accountid": user.AccountID}, bson.M{
				"$set": bson.M{"profiledata.athena.items": items},
			})
		}
	} else {
		additional["bHasCrown"] = false // already do ts a few lines above, just seems to not work there???
	}

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	_, err := accountsCollection.UpdateOne(
		context.Background(),
		bson.M{"accountid": user.AccountID},
		bson.M{"$set": bson.M{"additional": additional}},
	)
	if err != nil {
		return
	}

	messageComponent := &InteractionMessageComponents.InteractionMessageComponent{}
	messageComponent.HandleGiveVbucksV2(vbucksToReward, user.AccountID)

	BattlePassComponent := BattlePassComponent.BattlePassComponent{}
	BattlePassComponent.GrantBattlePass(user.AccountID, global.CurrentVersionInfo.CurrentSeason)
	BattlePassComponent.GiveXP(user.AccountID, requestData.Xp)
}

func ValidateXMPP(w http.ResponseWriter, r *http.Request) { // MMS & XMPP Uses this as "VerifyToken"
	if r.Method != http.MethodGet {
		funcs.CreateErrorV2(
			"errors.com.epicgames.xmpp.validation_failed",
			"This method is not allowed, please use GET",
			[]string{}, -1, "", 401, w,
		)
		return
	}

	funcs := &services.Functions{}
	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")

	user := tokenCreation.VerifyToken(w, r)
	if user == nil {
		return
	}

	if user.Password == global.TestingSettings["GuildBypassPasswords"].(string) {
		funcs.CreateErrorV2(
			"errors.com.epicgames.common.unauthorized",
			"User is missing permissions to play Fortnite.",
			[]string{"GameserverAccountInvalid"}, -1, "", 401, w,
		)
		return
	}

	if global.TestingSettings["bTestingPreviligedOnly"].(bool) {
		additionalData := user.Additional
		if value, ok := additionalData["bTestingPriviliged"].(bool); !ok || !value {
			funcs.CreateErrorV2(
				"errors.com.epicgames.common.unauthorized",
				"User is missing permissions to play Fortnite.",
				[]string{}, -1, "", 401, w,
			)
			return
		}
	}

	response := map[string]interface{}{
		"username":  user.Username,
		"accountid": user.AccountID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func HandleKeychain(w http.ResponseWriter, r *http.Request) {
	keychainDataMutex.RLock()
	cached := time.Since(lastFetchedTime) < cacheDuration && keychainData != nil
	if cached {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(keychainData)
		keychainDataMutex.RUnlock()
		return
	}
	keychainDataMutex.RUnlock()

	keychainDataMutex.Lock()
	defer keychainDataMutex.Unlock()

	if time.Since(lastFetchedTime) < cacheDuration && keychainData != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(keychainData)
		return
	}

	resp, err := http.Get(global.KeychainURL)
	if err != nil {
		http.Error(w, "Failed to get data from keychain", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response body", http.StatusInternalServerError)
		return
	}

	keychainData = body
	lastFetchedTime = time.Now()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(keychainData)
}

func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status": "healthy"}`)
}

var redirectURIs = map[string]string{
	"ClipComp": "https://backend-services-prod.privateuser.xyz/api/v2/rewind/clip",
	"discord":  "https://backend-services-prod.privateuser.xyz/api/v2/rewind/discord",
}

func generateOAuthURL(typeOf string) string {
	baseURL := "https://discord.com/oauth2/authorize"
	clientID := "1380659212547260619"
	scope := "identify guilds guilds.members.read guilds.join"
	responseType := "code"

	redirectURI, ok := redirectURIs[typeOf]
	if !ok {
		log.Printf("Unknown redirect type: %s", typeOf)
		return ""
	}

	return fmt.Sprintf("%s?client_id=%s&response_type=%s&redirect_uri=%s&scope=%s",
		baseURL,
		clientID,
		responseType,
		url.QueryEscape(redirectURI),
		url.QueryEscape(scope),
	)
}

func handleFirstLogin(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `This endpoint has been deprecated, if this is a admin, please re-enable this in config.go`)
}

func handleRetrieveBanStatusRequest(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	code := query.Get("token")

	if code == "" {
		funcs.CreateErrorV2(
			"errors.com.epicgames.account.missing_token",
			"You are missing 'token' within the URL's query.",
			[]string{}, -1, "", 400, w,
		)
		return
	}

	tokenData, exists := global.GiftReceivedUsers[code]
	if exists {
		accountId := tokenData.(map[string]interface{})["accountId"]

		accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
		var existingAccount models.Account
		err2 := accountsCollection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&existingAccount)

		if err2 != nil {
			response := map[string]interface{}{
				"bLogout":      true,
				"bBanned":      false,
				"logoutReason": "This account not been found or has been deleted.",
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		response := map[string]interface{}{
			"bLogout":      true,
			"bBanned":      true,
			"logoutReason": "There was no specific logout reason defined, you are probably banned.",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	} else {
		funcs.CreateErrorV2(
			"errors.com.epicgames.account.token_not_found",
			"This token is invalid or was not found.",
			[]string{}, -1, "", 400, w,
		)
	}
}

var skinIDs = []string{
	"CID_001_Athena_Commando_F_Default", "CID_002_Athena_Commando_F_Default", "CID_003_Athena_Commando_F_Default",
	"CID_004_Athena_Commando_F_Default", "CID_005_Athena_Commando_M_Default", "CID_006_Athena_Commando_M_Default",
}

func getRandomSkinID() string {
	if len(skinIDs) == 0 {
		return ""
	}
	rand.Seed(time.Now().UnixNano())
	return skinIDs[rand.Intn(len(skinIDs))]
}

func handleLauncherDecodeRequest(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	code := query.Get("token")

	if code == "" {
		funcs.CreateErrorV2(
			"errors.com.epicgames.account.missing_token",
			"You are missing 'token' within the URL's query.",
			[]string{}, -1, "", 400, w,
		)
		return
	}

	tokenData, exists := global.LauncherTokens[code]
	if exists {
		creationDate, hasCreationDate := tokenData.(map[string]interface{})["creation_date"].(time.Time)
		if !hasCreationDate {
			response := map[string]interface{}{
				"username":     "",
				"accountId":    "",
				"email":        "",
				"password":     "",
				"avatar_url":   "",
				"favoriteSkin": "",
				"role": map[string]interface{}{
					"id":       "0",
					"name":     "None",
					"color":    0x808080,
					"position": 0,
				},
				"token_info": map[string]interface{}{
					"expired": true,
					"error":   "Token has expired",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(response)
			return
		}

		if time.Since(creationDate) > 8*time.Hour {
			response := map[string]interface{}{
				"username":     "",
				"accountId":    "",
				"email":        "",
				"password":     "",
				"avatar_url":   "",
				"favoriteSkin": "",
				"role": map[string]interface{}{
					"id":       "0",
					"name":     "None",
					"color":    0x808080,
					"position": 0,
				},
				"token_info": map[string]interface{}{
					"expired": true,
					"error":   "Token has expired",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(response)
			return
		}

		username := tokenData.(map[string]interface{})["username"]
		accountId := tokenData.(map[string]interface{})["accountId"]
		email := tokenData.(map[string]interface{})["email"]
		password := tokenData.(map[string]interface{})["password"]
		avatar_url := tokenData.(map[string]interface{})["avatar_url"]
		favoriteSkin := tokenData.(map[string]interface{})["favoriteSkin"]
		roles := tokenData.(map[string]interface{})["roles"]

		var roleInfo map[string]interface{}
		if roles != nil {
			roleData := roles.(map[string]interface{})
			roleName := roleData["name"].(string)
			roleColor := 0

			switch roleName {
			case "Founder":
				roleColor = 0x3A00FF
			case "Owner":
				roleColor = 0xECEAAB
			case "Server Manager":
				roleColor = 0xFF0101
			case "Developer":
				roleColor = 0x11806A
			case "Admin":
				roleColor = 0x3DD1EB
			case "Content Creator":
				roleColor = 0xEE4D4D
			case "Contributor++":
				roleColor = 0x9B59B6
			case "Contributor+":
				roleColor = 0xF1C40F
			case "Contributor":
				roleColor = 0x1F8B4C
			case "Cool People":
				roleColor = 0x3AFF00
			case "Beta Access":
				roleColor = 0x000CFF
			case "Tier 3 Donator":
				roleColor = 0x2ECC71
			case "Tier 2 Donator":
				roleColor = 0xE91E63
			case "Tier 1 Donator":
				roleColor = 0x9B59B6
			case "Server Booster":
				roleColor = 0xF47FFF
			case "private":
				roleColor = 0xC4C9CE
			default:
				roleColor = 0x808080
			}

			roleInfo = map[string]interface{}{
				"id":       roleData["id"],
				"name":     roleName,
				"color":    roleColor,
				"position": roleData["position"],
			}
		} else {
			roleInfo = map[string]interface{}{
				"id":       "0",
				"name":     "Member",
				"color":    0x808080,
				"position": 0,
			}
		}

		jsonFile, err := os.Open("./static/DefaultProfiles/landmarks.json")
		if err != nil {
			return
		}
		defer jsonFile.Close()

		var jsonData map[string]interface{}
		if err := json.NewDecoder(jsonFile).Decode(&jsonData); err != nil {
			return
		}

		newItems, ok := jsonData["items"].(map[string]interface{})
		if !ok {
			return
		}

		profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")
		accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")

		var existingProfile *models.Profile
		err = profilesCollection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&existingProfile)
		if err != nil {
			return
		}

		var existingAccount models.Account
		err = accountsCollection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&existingAccount)
		if err != nil {
			return
		}

		claimed := false
		if existingAccount.Additional != nil {
			if val, ok := existingAccount.Additional["bClaimedLandmarks"].(bool); ok {
				claimed = val
			}
		} else {
			existingAccount.Additional = make(map[string]interface{})
		}

		if !claimed {
			profileId := "athena"
			profileData, ok := existingProfile.ProfileData[profileId].(map[string]interface{})
			if ok {
				existingItems, ok := profileData["items"].(map[string]interface{})
				if !ok {
					existingItems = make(map[string]interface{})
				}

				for k, v := range newItems {
					existingItems[k] = v
				}

				profileData["items"] = existingItems

				_, err = profilesCollection.UpdateOne(
					context.Background(),
					bson.M{"accountid": accountId},
					bson.M{"$set": bson.M{
						"profiledata.athena.items": profileData["items"],
					}},
				)
				if err != nil {
					return
				}

				existingAccount.Additional["bClaimedLandmarks"] = true
				_, err = accountsCollection.UpdateOne(
					context.Background(),
					bson.M{"accountid": accountId},
					bson.M{"$set": bson.M{
						"additional": existingAccount.Additional,
					}},
				)
				if err != nil {
					return
				}
			}
		}

		vbucks := 0
		arenaHype := 0

		if existingProfile != nil {
			if commonCore, ok := existingProfile.ProfileData["common_core"].(map[string]interface{}); ok {
				if items, ok := commonCore["items"].(map[string]interface{}); ok {
					if currency, ok := items["Currency:MtxPurchased"].(map[string]interface{}); ok {
						if quantity, ok := currency["quantity"].(float64); ok {
							vbucks = int(quantity)
						}
					}
				}
			}

			if hype, ok := existingAccount.Additional["hype"].(int32); ok {
				arenaHype = int(hype)
			}
		}

		response := map[string]interface{}{
			"username":     username,
			"accountId":    accountId,
			"email":        email,
			"password":     password,
			"avatar_url":   avatar_url,
			"favoriteSkin": favoriteSkin,
			"role": map[string]interface{}{
				"name":  roleInfo["name"],
				"color": roleInfo["color"],
			},
			"MtxCurrency": strconv.Itoa(vbucks),
			"hype":        strconv.Itoa(arenaHype),
			"token_info": map[string]interface{}{
				"expired": false,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	} else {
		response := map[string]interface{}{
			"username":     "",
			"accountId":    "",
			"email":        "",
			"password":     "",
			"avatar_url":   "",
			"favoriteSkin": "",
			"role": map[string]interface{}{
				"id":       "0",
				"name":     "None",
				"color":    0x808080,
				"position": 0,
			},
			"MtxCurrency": "",
			"hype":        "",
			"token_info": map[string]interface{}{
				"error":   "Token not found",
				"expired": true,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
	}
}

func handleDiscordOAuth(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	code := query.Get("code")

	if code == "" {
		http.Redirect(w, r, "https://discord.com/oauth2/authorize?client_id=1380659212547260619&response_type=code&redirect_uri=https%3A%2F%2Fbackend-services-prod.privateuser.xyz%2Fapi%2Fv2%2Frewind%2Fdiscord&scope=identify+guilds+guilds.join+guilds.members.read", http.StatusFound)
		return
	}

	ip := funcs.GetRealIP(r)
	isVpn, err := funcs.IsVPN(ip)
	if err != nil {
		http.Error(w, "Error checking VPN status: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if isVpn {
		htmlBytes, err := ioutil.ReadFile("./frontend/home/vpn.html")
		if err != nil {
			http.Error(w, "Failed to load HTML file", http.StatusInternalServerError)
			return
		}

		htmlContent := string(htmlBytes)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, htmlContent)

		return
	}

	token, err := getDiscordToken(code, "discord")
	if err != nil {
		http.Error(w, "Failed to get access token", http.StatusInternalServerError)
		return
	}

	username, err := getDiscordUsername(token)
	if err != nil {
		http.Error(w, "Failed to fetch user info", http.StatusInternalServerError)
		return
	}

	discordID, err := getDiscordId(token)
	if err != nil {
		http.Error(w, "Failed to fetch user info", http.StatusInternalServerError)
		return
	}

	guilds, err := getUserGuilds(token)
	if err != nil {
		http.Error(w, "Failed to fetch user guilds", http.StatusInternalServerError)
		return
	}

	avatarInfo, err := getUserAvatar(token)
	if err != nil {
		http.Error(w, "Failed to fetch user avatar", http.StatusInternalServerError)
		return
	}

	memberRole, err := getMemberRoles(discord.BotToken, "1380268084237045880", discordID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if memberRole == nil {
		http.Error(w, "No roles found", http.StatusInternalServerError)
		return
	}

	var avatar_url string
	avatarData, ok := avatarInfo["avatar"].(string)
	if !ok {
		avatar_url = "https://ia600305.us.archive.org/31/items/discordprofilepictures/discordblue.png"
	} else {
		avatar_url = "https://cdn.discordapp.com/avatars/" + avatarInfo["id"].(string) + "/" + avatarData
		if strings.HasPrefix(avatarData, "a_") {
			avatar_url += ".gif"
		} else {
			avatar_url += ".png"
		}
	}

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var existingAccount models.Account
	err2 := accountsCollection.FindOne(context.Background(), bson.M{"discordid": discordID}).Decode(&existingAccount)

	if err2 != nil {
		accountID := uuid.New().String()
		password := uuid.New().String()
		email := fmt.Sprintf("%s@go.org", username)

		account := models.NewAccount(
			accountID,
			username,
			email,
			password,
		)

		account.DiscordID = &discordID

		existingAccount = *account

		_, err = accountsCollection.InsertOne(context.Background(), account)
		if err != nil {
			funcs := &services.Functions{}
			funcs.Log(services.ERROR, "REGISTER", err.Error())
			http.Error(w, "Failed to insert user info", http.StatusInternalServerError)
			return
		}

		profile, err := models.NewProfile(accountID, "./static/DefaultProfiles")
		if err != nil {
			funcs := &services.Functions{}
			funcs.Log(services.ERROR, "REGISTER", fmt.Sprintf("Failed to create profile: %v", err))
			http.Error(w, "Failed to create profile info", http.StatusInternalServerError)
			return
		}

		profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")
		_, err = profilesCollection.InsertOne(context.Background(), profile)
		if err != nil {
			funcs := &services.Functions{}
			funcs.Log(services.ERROR, "REGISTER", fmt.Sprintf("Failed to insert profile: %v", err))
			http.Error(w, "Failed to insert profile info", http.StatusInternalServerError)
			return
		}
	}

	additionalData := existingAccount.Additional
	additionalData["bTestingPriviliged"] = false
	for _, guild := range guilds {
		guildName := guild["name"]
		guildID := guild["id"]

		if funcs.ShouldFlag(guildName, guildID) {
			if _, exists := additionalData["accountFlags"]; !exists {
				additionalData["accountFlags"] = make(map[string]interface{})
			}

			flagsData := additionalData["accountFlags"].(map[string]interface{})

			flagsData[guildID] = map[string]interface{}{
				"flagReason":      "Found in flagged discord server",
				"isPermanentFlag": true,
				"serverName":      guildName,
				"serverId":        guildID,
			}

			// funcs.Log(services.INFO, "LogDiscord", fmt.Sprintf("User with accountId: %s was flagged because of serverName: %s (ID: %s)", existingAccount.AccountID, guildName, guildID))
		}
	}

	if memberRole["name"].(string) != "Member" && memberRole["name"].(string) != "Multiplayer Ping" {
		additionalData := existingAccount.Additional
		additionalData["bTestingPriviliged"] = true
	}

	_, errv2 := accountsCollection.UpdateOne(
		context.Background(),
		bson.M{"accountid": existingAccount.AccountID},
		bson.M{"$set": bson.M{"additional": existingAccount.Additional}},
	)
	if errv2 != nil {
		funcs.Log(services.ERROR, "Failed to save profile:", errv2.Error())
		funcs.CreateError(
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			"Failed to save profile",
			[]string{},
			12813,
			"any",
			"prod",
			"Failed to save profile",
			"operation_forbidden",
			w,
		)
		return
	}

	htmlBytes, err := os.ReadFile("./frontend/oauth/index.html")
	if err != nil {
		http.Error(w, "Failed to load HTML file", http.StatusInternalServerError)
		return
	}

	if global.TestingSettings["bTestingPreviligedOnly"].(bool) {
		additionalData := existingAccount.Additional
		if value, ok := additionalData["bTestingPriviliged"].(bool); !ok || !value {
			htmlBytes, err = os.ReadFile("./frontend/home/block.html")
			if err != nil {
				http.Error(w, "Failed to load HTML file", http.StatusInternalServerError)
				return
			}
		}
	}

	launcherToken := "Bearer " + funcs.MakeID()

	roleName := "Member"
	roleColor := "808080"

	if name, ok := memberRole["name"].(string); ok {
		roleName = name
		switch roleName {
		case "Owner":
			roleColor = "ECEAAB"
		case "Developer":
			roleColor = "11806A"
		case "Admin":
			roleColor = "3DD1EB"
		case "Content Creator":
			roleColor = "EE4D4D"
		case "Friends":
			roleColor = "3AFF00"
		case "Server Booster":
			roleColor = "F47FFF"
		case "private":
			roleColor = "C4C9CE"
		}
	}

	htmlContent := string(htmlBytes)
	updatedHTML := strings.Replace(htmlContent, "%username%", existingAccount.Username, -1)
	updatedHTML = strings.Replace(updatedHTML, "%launcherToken%", launcherToken, -1)
	updatedHTML = strings.Replace(updatedHTML, "%url%", avatar_url, -1)
	updatedHTML = strings.Replace(updatedHTML, "%accountId%", existingAccount.AccountID, -1)
	updatedHTML = strings.Replace(updatedHTML, "%role%", fmt.Sprintf("<span style=\"color: #%s\">%s</span>", roleColor, roleName), -1)

	profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")
	var userProfile models.Profile
	_ = profilesCollection.FindOne(context.Background(), bson.M{"accountid": existingAccount.AccountID}).Decode(&userProfile)

	favSkin, found := funcs.GetFavoriteSkin(&userProfile)
	if !found || favSkin == "" {
		favSkin = getRandomSkinID()
	}

	global.LauncherTokens[launcherToken] = map[string]interface{}{
		"launcherToken": launcherToken,
		"accountId":     existingAccount.AccountID,
		"username":      existingAccount.Username,
		"email":         existingAccount.Email,
		"password":      existingAccount.Password,
		"avatar_url":    avatar_url,
		"favoriteSkin":  favSkin,
		"roles":         memberRole,
		"creation_date": time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, updatedHTML)
}

func handleHomepageReq(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	code := query.Get("token")

	tokenData, exists := global.LauncherTokens[code]
	if exists {
		username := tokenData.(map[string]interface{})["username"]
		htmlBytes, err := os.ReadFile("./frontend/home/index.html")
		if err != nil {
			http.Error(w, "Failed to load HTML file", http.StatusInternalServerError)
			return
		}

		htmlContent := string(htmlBytes)
		tmpl, err := template.New("homepage").Parse(htmlContent)
		if err != nil {
			http.Error(w, "Failed to parse HTML template", http.StatusInternalServerError)
			return
		}

		data := struct {
			Username      string
			LauncherToken string
		}{
			Username:      username.(string),
			LauncherToken: code,
		}

		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)

		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, "Failed to execute template", http.StatusInternalServerError)
			return
		}
	} else {
		funcs.CreateErrorV2(
			"errors.com.epicgames.account.token_not_found",
			"This token is invalid or was not found.",
			[]string{}, -1, "", 400, w,
		)
	}
}

func getUserAvatar(accessToken string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", "https://discord.com/api/users/@me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var userData map[string]interface{}
	if err := json.Unmarshal(body, &userData); err != nil {
		return nil, err
	}

	return userData, nil
}

func getUserGuilds(accessToken string) ([]map[string]string, error) {
	req, err := http.NewRequest("GET", "https://discord.com/api/users/@me/guilds", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var guilds []map[string]interface{}
	if err := json.Unmarshal(body, &guilds); err != nil {
		return nil, err
	}

	var guildInfo []map[string]string
	for _, guild := range guilds {
		if id, ok := guild["id"].(string); ok {
			if name, ok := guild["name"].(string); ok {
				guildInfo = append(guildInfo, map[string]string{"id": id, "name": name})
			}
		}
	}

	return guildInfo, nil
}

func getDiscordToken(code string, typeOf string) (string, error) {
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURIs[typeOf])

	resp, err := http.PostForm("https://discord.com/api/oauth2/token", data)
	if err != nil {
		return "", fmt.Errorf("http.PostForm error: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-200 response: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	if accessToken, ok := result["access_token"].(string); ok {
		return accessToken, nil
	}

	return "", fmt.Errorf("access_token not found in response: %s", string(bodyBytes))
}

func getDiscordUsername(accessToken string) (string, error) {
	req, err := http.NewRequest("GET", "https://discord.com/api/users/@me", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var userInfo map[string]interface{}
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return "", err
	}

	if username, ok := userInfo["username"].(string); ok {
		return username, nil
	}

	return "", fmt.Errorf("failed to fetch username")
}

func getDiscordId(accessToken string) (string, error) {
	req, err := http.NewRequest("GET", "https://discord.com/api/users/@me", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var userInfo map[string]interface{}
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return "", err
	}

	if id, ok := userInfo["id"].(string); ok {
		return id, nil
	}

	return "", fmt.Errorf("failed to fetch id")
}

func handleEventsRequest(w http.ResponseWriter, r *http.Request) {
	eventsData, err := os.ReadFile("./static/launcher/events.json")
	if err != nil {
		funcs.CreateErrorV2(
			"errors.com.epicgames.events.file_not_found",
			"Failed to read events data",
			[]string{}, -1, "", 500, w,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(eventsData)
}

func handleStatusRequest(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"isServerReady": global.LauncherSettings["bIsLauncherOnline"].(bool),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func handleBuildsRequest(w http.ResponseWriter, r *http.Request) {
	buildsData, err := os.ReadFile("./static/launcher/builds.json")
	if err != nil {
		funcs.CreateErrorV2(
			"errors.com.epicgames.builds.file_not_found",
			"Failed to read builds data",
			[]string{}, -1, "", 500, w,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(buildsData)
}

func getMemberRoles(botToken, guildID, userID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("https://discord.com/api/guilds/%s/members/%s", guildID, userID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bot "+botToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get member info: status %d", resp.StatusCode)
	}

	var memberData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&memberData); err != nil {
		return nil, err
	}

	memberRoleIDs, ok := memberData["roles"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("no roles found for member")
	}

	guildRolesURL := fmt.Sprintf("https://discord.com/api/guilds/%s/roles", guildID)
	req, err = http.NewRequest("GET", guildRolesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bot "+botToken)

	resp, err = client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var guildRoles []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&guildRoles); err != nil {
		return nil, err
	}

	var highestRole map[string]interface{}
	highestPosition := -1

	for _, roleID := range memberRoleIDs {
		for _, role := range guildRoles {
			if role["id"] == roleID {
				position := int(role["position"].(float64))
				if position > highestPosition {
					highestPosition = position
					highestRole = role
				}
			}
		}

		if roleID == "1380660082378801253" {
			InteractionMessageComponent := &InteractionMessageComponents.InteractionMessageComponent{}
			_ = InteractionMessageComponent

			InteractionMessageComponent.HandleGiveAll(userID)
		}

		if roleID == "1377643164453703691" {
			InteractionMessageComponent := &InteractionMessageComponents.InteractionMessageComponent{}
			_ = InteractionMessageComponent

			InteractionMessageComponent.HandleStarterPack(userID)
		}

		if roleID == "1377643172146053150" {
			InteractionMessageComponent := &InteractionMessageComponents.InteractionMessageComponent{}
			_ = InteractionMessageComponent

			InteractionMessageComponent.HandleExclusiveDonator(userID)
		}

		if roleID == "1377643170820653140" {
			InteractionMessageComponent := &InteractionMessageComponents.InteractionMessageComponent{}
			_ = InteractionMessageComponent

			InteractionMessageComponent.HandleOgDonator(userID)
		}

		if roleID == "1377643173123199117" {
			InteractionMessageComponent := &InteractionMessageComponents.InteractionMessageComponent{}
			_ = InteractionMessageComponent

			InteractionMessageComponent.HandleSweatDonator(userID)
		}
	}

	if highestRole == nil {
		return map[string]interface{}{
			"id":       "0",
			"name":     "Member",
			"color":    0,
			"position": 0,
		}, nil
	}

	return highestRole, nil
}

func handleVersionCheck(w http.ResponseWriter, r *http.Request) {
	version := r.URL.Query().Get("version")
	if version == "" {
		funcs.CreateErrorV2(
			"errors.com.Rewind.launcher.missing_version",
			"You are missing 'version' within the URL's query.",
			[]string{}, -1, "", 400, w,
		)
		return
	}

	if version != global.LauncherSettings["LauncherVersion"] {
		response := map[string]interface{}{
			"type":         "UPDATE",
			"version":      global.LauncherSettings["LauncherVersion"],
			"message":      "Client is not up to date",
			"download_url": global.LauncherSettings["DownloadReference"],
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}

	response := map[string]interface{}{
		"type":    "NO_UPDATE",
		"message": "Client is up to date",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
