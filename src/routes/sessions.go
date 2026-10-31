package routes

import (
	"encoding/hex"
	"encoding/json"
	"go-backend/src/global"
	"go-backend/src/services"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
)

type Session = global.Session

func InitGameSessions() {
	http.HandleFunc("/fortnite/api/matchmaking/session", HandleMatchmakingSession_POST)
	http.HandleFunc("/fortnite/api/game/v2/matchmakingservice/ticket/session/{sessionId}", SessionServiceTicket)
	http.HandleFunc("/fortnite/api/matchmaking/session/{sessionId}/players", HandleGameSessionPlayers)
	http.HandleFunc("/fortnite/api/matchmaking/session/{sessionId}/heartbeat", Heartbeat)
	http.HandleFunc("/fortnite/api/matchmaking/session/{sessionId}", HandleSessionReq)

	http.HandleFunc("/fortnite/api/v2/matchmaking/session/", HandleSessionDataMMS)
}

func HandleSessionDataMMS(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Path
	sessionId := strings.TrimPrefix(rawPath, "/fortnite/api/v2/matchmaking/session/")

	if sessionId == "" {
		funcs.Log(services.ERROR, "LogSession", "Invalid URL structure!")
		http.Error(w, "Invalid URL structure. Expected format: /fortnite/api/matchmaking/session/{sessionId}", http.StatusBadRequest)
		return
	}

	session, ok := global.Sessions[sessionId]
	if !ok {
		funcs.Log(services.ERROR, "LogSession", "Session not found!")
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"owner":      session.OwnerId,
		"playlistId": session.PlaylistName,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func HandleSessionReq(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Path
	sessionId := strings.TrimPrefix(rawPath, "/fortnite/api/matchmaking/session/")
	parts := strings.Split(sessionId, "/")

	if len(parts) < 1 || parts[0] == "" {
		http.Error(w, "Invalid URL structure. Expected format: /fortnite/api/matchmaking/session/{sessionId}/players", http.StatusBadRequest)
		return
	}
	sessionId = parts[0]

	session := global.Sessions[sessionId]

	if r.Method == http.MethodPost {
		handlePostPlayers(w, r, session)
	} else if r.Method == http.MethodGet {
		HandleJoinSession(w, r)
	} else if r.Method == http.MethodPut {
		HandleJoinSession(w, r)
	} else {
		funcs.Log(services.WARNING, "LogSession", "Missing Method / Method disabled: "+r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func HandleUpdateSession(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Path
	sessionId := strings.TrimPrefix(rawPath, "/fortnite/api/matchmaking/session/")

	if sessionId == "" {
		funcs.Log(services.ERROR, "LogSession", "Invalid URL structure!")
		http.Error(w, "Invalid URL structure. Expected format: /fortnite/api/matchmaking/session/{sessionId}", http.StatusBadRequest)
		return
	}

	session, ok := global.Sessions[sessionId]
	if !ok {
		funcs.Log(services.ERROR, "LogSession", "Session not found!")
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	var body map[string]interface{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&body); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	publicPlayers, privatePlayers := parsePlayers(body)
	attributesStr := parseAttributes(body)
	sessionUpdated := createSession(sessionId, body, publicPlayers, privatePlayers, attributesStr, r)
	global.Sessions[sessionId] = sessionUpdated
	response := prepareResponse(session)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func HandleJoinSession(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Path
	sessionId := strings.TrimPrefix(rawPath, "/fortnite/api/matchmaking/session/")

	if sessionId == "" {
		funcs.Log(services.ERROR, "LogSession", "Invalid URL structure!")
		http.Error(w, "Invalid URL structure. Expected format: /fortnite/api/matchmaking/session/{sessionId}", http.StatusBadRequest)
		return
	}

	if sessionId == "yap" {
		response := map[string]interface{}{
			"id":                 "yap",
			"sessionId":          "yap",
			"ownerId":            strings.ToUpper(strings.ReplaceAll(funcs.MakeID(), "-", "")),
			"ownerName":          "[DS]fortnite-liveeuaws02c59ubrcore-4821335-2-i-034a139c3e237c6a1-5626",
			"serverName":         "[DS]fortnite-liveeuaws02c59ubrcore-4821335-2-i-034a139c3e237c6a1-5626",
			"serverAddress":      "127.0.0.1",
			"serverPort":         7777,
			"maxPublicPlayers":   120,
			"openPublicPlayers":  23,
			"maxPrivatePlayers":  0,
			"openPrivatePlayers": 0,
			"attributes": map[string]interface{}{
				"REGION_s":          "EU",
				"GAMEMODE_s":        "FORTATHENA",
				"SUBREGION_s":       "DE",
				"DCID_s":            "FORTNITE-LIVEEUAWS02C59UBRCORE-4821335-2",
				"NEEDS_i":           2,
				"NEEDSSORT_i":       2,
				"tenant_s":          "Fortnite",
				"MATCHMAKINGPOOL_s": "Any",
				"PLAYLISTID_i":      2,
				"HOTFIXVERSION_i":   0,
				"SESSIONKEY_s":      "DA80811808D685E6C02FE6D842502312",
				"TENANT_s":          "Fortnite",
				"BEACONPORT_i":      7777,
			},
			"publicPlayers":                   []interface{}{},
			"privatePlayers":                  []interface{}{},
			"totalPlayers":                    97,
			"allowJoinInProgress":             false,
			"shouldAdvertise":                 false,
			"isDedicated":                     false,
			"usesStats":                       false,
			"allowInvites":                    false,
			"usesPresence":                    false,
			"allowJoinViaPresence":            true,
			"allowJoinViaPresenceFriendsOnly": false,
			"buildUniqueId":                   "0",
			"lastUpdated":                     time.Now().UTC().Format(time.RFC3339),
			"started":                         false,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
		return
	}

	session, ok := global.Sessions[sessionId]
	if !ok {
		funcs.Log(services.ERROR, "LogSession", "Session not found!")
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	host, port, err := net.SplitHostPort(session.ServerAddress)
	if err != nil {
		host = session.ServerAddress
		port = ""
	}

	if global.MatchmakingSettings["bForceBeaconPort"].(bool) {
		session.Attributes["BEACONPORT_i"] = port
	}

	response := map[string]interface{}{
		"id":                              session.SessionId,
		"sessionId":                       session.SessionId,
		"serverAddress":                   host,
		"lastUpdated":                     session.LastUpdated,
		"ownerId":                         session.OwnerId,
		"ownerName":                       session.OwnerName,
		"serverName":                      session.ServerName,
		"maxPublicPlayers":                session.MaxPublicPlayers,
		"maxPrivatePlayers":               session.MaxPrivatePlayers,
		"shouldAdvertise":                 session.ShouldAdvertise,
		"allowJoinInProgress":             session.AllowJoinInProgress,
		"isDedicated":                     session.IsDedicated,
		"usesStats":                       session.UsesStats,
		"allowInvites":                    session.AllowInvites,
		"usesPresence":                    session.UsesPresence,
		"allowJoinViaPresence":            session.AllowJoinViaPresence,
		"allowJoinViaPresenceFriendsOnly": session.AllowJoinViaPresenceFriendsOnly,
		"buildUniqueId":                   session.BuildUniqueId,
		"attributes":                      session.Attributes,
		"serverPort":                      session.ServerPort,
		"openPublicPlayers":               session.OpenPublicPlayers,
		"openPrivatePlayers":              session.OpenPrivatePlayers,
		"sortWeight":                      session.SortWeight,
		"started":                         session.Started,
		"publicPlayers":                   session.PublicPlayers,
		"privatePlayers":                  session.PrivatePlayers,
		"stopped":                         session.Stopped,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func HandleGameSessionPlayers(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Path
	sessionId := strings.TrimPrefix(rawPath, "/fortnite/api/matchmaking/session/")
	parts := strings.Split(sessionId, "/")

	if len(parts) < 1 || parts[0] == "" {
		http.Error(w, "Invalid URL structure. Expected format: /fortnite/api/matchmaking/session/{sessionId}/players", http.StatusBadRequest)
		return
	}
	sessionId = parts[0]
	session := global.Sessions[sessionId]

	if r.Method == http.MethodPost {
		handlePostPlayers(w, r, session)
	} else if r.Method == http.MethodDelete {
		handleDeletePlayers(w, r, session)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handlePostPlayers(w http.ResponseWriter, r *http.Request, session Session) {
	var body map[string]interface{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&body); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	publicPlayersInterface, ok := body["publicPlayers"].([]interface{})
	if !ok {
		http.Error(w, "Invalid publicPlayers format", http.StatusBadRequest)
		return
	}
	privatePlayersInterface, ok := body["privatePlayers"].([]interface{})
	if !ok {
		http.Error(w, "Invalid privatePlayers format", http.StatusBadRequest)
		return
	}

	var newPublicPlayers []string
	for _, player := range publicPlayersInterface {
		if str, ok := player.(string); ok {
			newPublicPlayers = append(newPublicPlayers, str)
		}
	}

	var newPrivatePlayers []string
	for _, player := range privatePlayersInterface {
		if str, ok := player.(string); ok {
			newPrivatePlayers = append(newPrivatePlayers, str)
		}
	}

	publicList := append(session.PublicPlayers, newPublicPlayers...)
	privateList := append(session.PrivatePlayers, newPrivatePlayers...)

	publicPlayersMap := make(map[string]struct{})
	for _, player := range publicList {
		publicPlayersMap[player] = struct{}{}
	}
	privatePlayersMap := make(map[string]struct{})
	for _, player := range privateList {
		privatePlayersMap[player] = struct{}{}
	}

	var distinctPublicPlayers []string
	for player := range publicPlayersMap {
		distinctPublicPlayers = append(distinctPublicPlayers, player)
	}

	var distinctPrivatePlayers []string
	for player := range privatePlayersMap {
		distinctPrivatePlayers = append(distinctPrivatePlayers, player)
	}

	if len(distinctPublicPlayers) == 0 {
		distinctPublicPlayers = []string{}
	}
	if len(distinctPrivatePlayers) == 0 {
		distinctPrivatePlayers = []string{}
	}

	session.PublicPlayers = distinctPublicPlayers
	session.PrivatePlayers = distinctPrivatePlayers

	session.LastUpdated = time.Now().UTC().Format(time.RFC3339)
	session.OpenPublicPlayers = len(session.PublicPlayers) - 1

	global.Sessions[session.SessionId] = session

	response := map[string]interface{}{
		"sessionId":                       session.SessionId,
		"serverAddress":                   session.ServerAddress,
		"lastUpdated":                     session.LastUpdated,
		"ownerId":                         session.OwnerId,
		"ownerName":                       session.OwnerName,
		"serverName":                      session.ServerName,
		"maxPublicPlayers":                session.MaxPublicPlayers,
		"maxPrivatePlayers":               session.MaxPrivatePlayers,
		"shouldAdvertise":                 session.ShouldAdvertise,
		"allowJoinInProgress":             session.AllowJoinInProgress,
		"isDedicated":                     session.IsDedicated,
		"usesStats":                       session.UsesStats,
		"allowInvites":                    session.AllowInvites,
		"usesPresence":                    session.UsesPresence,
		"allowJoinViaPresence":            session.AllowJoinViaPresence,
		"allowJoinViaPresenceFriendsOnly": session.AllowJoinViaPresenceFriendsOnly,
		"buildUniqueId":                   session.BuildUniqueId,
		"attributes":                      session.Attributes,
		"serverPort":                      session.ServerPort,
		"openPublicPlayers":               session.OpenPublicPlayers,
		"openPrivatePlayers":              session.OpenPrivatePlayers,
		"sortWeight":                      session.SortWeight,
		"started":                         session.Started,
		"publicPlayers":                   session.PublicPlayers,
		"privatePlayers":                  session.PrivatePlayers,
		"stopped":                         session.Stopped,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func handleDeletePlayers(w http.ResponseWriter, r *http.Request, session Session) {
	var body []string
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&body); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	session.PublicPlayers = removePlayers(session.PublicPlayers, body)
	session.LastUpdated = time.Now().UTC().Format(time.RFC3339)

	global.Sessions[session.SessionId] = session

	response := map[string]interface{}{
		"sessionId":                       session.SessionId,
		"serverAddress":                   session.ServerAddress,
		"lastUpdated":                     session.LastUpdated,
		"ownerId":                         session.OwnerId,
		"ownerName":                       session.OwnerName,
		"serverName":                      session.ServerName,
		"maxPublicPlayers":                session.MaxPublicPlayers,
		"maxPrivatePlayers":               session.MaxPrivatePlayers,
		"shouldAdvertise":                 session.ShouldAdvertise,
		"allowJoinInProgress":             session.AllowJoinInProgress,
		"isDedicated":                     session.IsDedicated,
		"usesStats":                       session.UsesStats,
		"allowInvites":                    session.AllowInvites,
		"usesPresence":                    session.UsesPresence,
		"allowJoinViaPresence":            session.AllowJoinViaPresence,
		"allowJoinViaPresenceFriendsOnly": session.AllowJoinViaPresenceFriendsOnly,
		"buildUniqueId":                   session.BuildUniqueId,
		"attributes":                      session.Attributes,
		"serverPort":                      session.ServerPort,
		"openPublicPlayers":               session.OpenPublicPlayers,
		"openPrivatePlayers":              session.OpenPrivatePlayers,
		"sortWeight":                      session.SortWeight,
		"started":                         session.Started,
		"publicPlayers":                   session.PublicPlayers,
		"privatePlayers":                  session.PrivatePlayers,
		"stopped":                         session.Stopped,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func removePlayers(players []string, playersToRemove []string) []string {
	playerMap := make(map[string]struct{})
	for _, player := range players {
		playerMap[player] = struct{}{}
	}
	for _, playerToRemove := range playersToRemove {
		delete(playerMap, playerToRemove)
	}

	var updatedPlayers []string
	for player := range playerMap {
		updatedPlayers = append(updatedPlayers, player)
	}
	return updatedPlayers
}

func SessionServiceTicket(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Path
	rawPath = strings.TrimPrefix(rawPath, "/fortnite/api/game/v2/matchmakingservice/ticket/session/")
	parts := strings.Split(rawPath, "/")

	if len(parts) < 1 || parts[0] == "" {
		funcs.Log(services.ERROR, "Invalid URL structure", rawPath)
		http.Error(w, "Invalid URL structure. Expected format: /fortnite/api/game/v2/matchmakingservice/ticket/session/{sessionId}", http.StatusBadRequest)
		return
	}

	sessionId := parts[0]
	if sessionId == "" {
		http.Error(w, "sessionId is required", http.StatusBadRequest)
		return
	}

	bucketIds := r.URL.Query().Get("bucketIds")
	if bucketIds == "" {
		http.Error(w, "bucketIds is required", http.StatusBadRequest)
		return
	}

	bucketIdParts := strings.Split(bucketIds, ":")
	if len(bucketIdParts) < 4 {
		http.Error(w, "Invalid bucketIds format", http.StatusBadRequest)
		return
	}

	finalBucketIds := strings.Join(bucketIdParts[:4], ":")

	randomBytes32 := make([]byte, 32)
	if _, err := rand.Read(randomBytes32); err != nil {
		http.Error(w, "Failed to generate random bytes", http.StatusInternalServerError)
		return
	}

	jti := hex.EncodeToString(randomBytes32)

	now := time.Now().UTC()
	expirationTime := now.Add(240 * time.Minute)

	ua := r.Header.Get("User-Agent")

	payload := jwt.MapClaims{
		"bucketId":      finalBucketIds,
		"region":        bucketIdParts[2],
		"version":       ua,
		"buildUniqueId": bucketIdParts[0],
		"exp":           expirationTime.Unix(),
		"iat":           now.Unix(),
		"jti":           jti,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, payload)

	secretKey := "SigmaJWT"
	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		http.Error(w, "Failed to create token", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"serviceUrl": "wss://private-session-service.privateuser.xyz",
		"ticketType": "mms-ticket",
		"payload":    sessionId,
		"signature":  tokenString,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func HandleMatchmakingSession_POST(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&body); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	sessionId := generateSessionID()
	publicPlayers, privatePlayers := parsePlayers(body)
	attributesStr := parseAttributes(body)
	session := createSession(sessionId, body, publicPlayers, privatePlayers, attributesStr, r)
	global.Sessions[sessionId] = session
	response := prepareResponse(session)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func Heartbeat(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Path
	rawPath = strings.TrimPrefix(rawPath, "/fortnite/api/matchmaking/session/")
	parts := strings.Split(rawPath, "/")

	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		http.Error(w, "Invalid URL structure. Expected format: /fortnite/api/matchmaking/session/{sessionId}/heartbeat", http.StatusBadRequest)
		return
	}

	sessionId := parts[0]
	if sessionId == "" {
		http.Error(w, "sessionId is required", http.StatusBadRequest)
		return
	}

	global.Mu.RLock()
	session, exists := global.Sessions[sessionId]
	global.Mu.RUnlock()

	if !exists {
		http.Error(w, "Unknown session", http.StatusNotFound)
		return
	}

	if session.ServerAddress == "" {
		session.ServerAddress = "127.0.0.1"
	}

	session.LastUpdated = time.Now().UTC().Format(time.RFC3339)

	global.Mu.Lock()
	global.Sessions[sessionId] = session
	global.Mu.Unlock()

	response := map[string]interface{}{
		"sessionId":                       session.SessionId,
		"serverAddress":                   session.ServerAddress,
		"lastUpdated":                     session.LastUpdated,
		"ownerId":                         session.OwnerId,
		"ownerName":                       session.OwnerName,
		"serverName":                      session.ServerName,
		"maxPublicPlayers":                session.MaxPublicPlayers,
		"maxPrivatePlayers":               session.MaxPrivatePlayers,
		"shouldAdvertise":                 session.ShouldAdvertise,
		"allowJoinInProgress":             session.AllowJoinInProgress,
		"isDedicated":                     session.IsDedicated,
		"usesStats":                       session.UsesStats,
		"allowInvites":                    session.AllowInvites,
		"usesPresence":                    session.UsesPresence,
		"allowJoinViaPresence":            session.AllowJoinViaPresence,
		"allowJoinViaPresenceFriendsOnly": session.AllowJoinViaPresenceFriendsOnly,
		"buildUniqueId":                   session.BuildUniqueId,
		"attributes":                      session.Attributes,
		"serverPort":                      session.ServerPort,
		"openPublicPlayers":               session.OpenPublicPlayers,
		"openPrivatePlayers":              session.OpenPrivatePlayers,
		"sortWeight":                      session.SortWeight,
		"started":                         session.Started,
		"publicPlayers":                   session.PublicPlayers,
		"privatePlayers":                  session.PrivatePlayers,
		"stopped":                         session.Stopped,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func generateSessionID() string {
	id := uuid.New().String()
	return id[0:8] + id[9:13] + id[14:18] + id[19:23] + id[24:]
}

func parsePlayers(body map[string]interface{}) ([]string, []string) {
	var publicPlayers []string
	var privatePlayers []string

	if pubPlayers, ok := body["publicPlayers"].([]interface{}); ok {
		for _, player := range pubPlayers {
			if playerStr, ok := player.(string); ok {
				publicPlayers = append(publicPlayers, playerStr)
			}
		}
	}

	if privPlayers, ok := body["privatePlayers"].([]interface{}); ok {
		for _, player := range privPlayers {
			if playerStr, ok := player.(string); ok {
				privatePlayers = append(privatePlayers, playerStr)
			}
		}
	}

	return publicPlayers, privatePlayers
}

func parseAttributes(body map[string]interface{}) map[string]interface{} {
	if attrs, ok := body["attributes"].(map[string]interface{}); ok {
		return attrs
	}
	return map[string]interface{}{}
}

func createSession(sessionId string, body map[string]interface{}, publicPlayers, privatePlayers []string, attributesStr map[string]interface{}, r *http.Request) Session {
	//funcs := &services.Functions{}
	//clientIP := funcs.GetClientIp(r)

	return Session{
		SessionId:                       sessionId,
		ServerAddress:                   r.RemoteAddr,
		LastUpdated:                     time.Now().UTC().Format(time.RFC3339),
		PlaylistName:                    getInt(body, "PLAYLISTID_i"),
		OwnerId:                         getString2(body, "ownerId"),
		OwnerName:                       getString2(body, "ownerName"),
		ServerName:                      getString2(body, "serverName"),
		MaxPublicPlayers:                getInt(body, "maxPublicPlayers"),
		MaxPrivatePlayers:               getInt(body, "maxPrivatePlayers"),
		ShouldAdvertise:                 getBool(body, "shouldAdvertise"),
		AllowJoinInProgress:             getBool(body, "allowJoinInProgress"),
		IsDedicated:                     getBool(body, "isDedicated"),
		UsesStats:                       getBool(body, "usesStats"),
		AllowInvites:                    getBool(body, "allowInvites"),
		UsesPresence:                    getBool(body, "usesPresence"),
		AllowJoinViaPresence:            getBool(body, "allowJoinViaPresence"),
		AllowJoinViaPresenceFriendsOnly: getBool(body, "allowJoinViaPresenceFriendsOnly"),
		BuildUniqueId:                   getString2(body, "buildUniqueId"),
		Attributes:                      attributesStr,
		ServerPort:                      getInt(body, "serverPort"),
		OpenPublicPlayers:               getInt(body, "openPublicPlayers"),
		OpenPrivatePlayers:              getInt(body, "openPrivatePlayers"),
		SortWeight:                      getInt(body, "sortWeight"),
		Started:                         getBool(body, "started"),
		PublicPlayers:                   publicPlayers,
		PrivatePlayers:                  privatePlayers,
		Stopped:                         false,
	}
}

func prepareResponse(session Session) map[string]interface{} {
	attributes := make(map[string]interface{})

	return map[string]interface{}{
		"id":                              session.SessionId,
		"serverAddress":                   session.ServerAddress,
		"lastUpdated":                     session.LastUpdated,
		"ownerId":                         session.OwnerId,
		"ownerName":                       session.OwnerName,
		"serverName":                      session.ServerName,
		"maxPublicPlayers":                session.MaxPublicPlayers,
		"maxPrivatePlayers":               session.MaxPrivatePlayers,
		"shouldAdvertise":                 session.ShouldAdvertise,
		"allowJoinInProgress":             session.AllowJoinInProgress,
		"isDedicated":                     session.IsDedicated,
		"usesStats":                       session.UsesStats,
		"allowInvites":                    session.AllowInvites,
		"usesPresence":                    session.UsesPresence,
		"allowJoinViaPresence":            session.AllowJoinViaPresence,
		"allowJoinViaPresenceFriendsOnly": session.AllowJoinViaPresenceFriendsOnly,
		"buildUniqueId":                   session.BuildUniqueId,
		"attributes":                      attributes,
		"serverPort":                      session.ServerPort,
		"openPublicPlayers":               session.OpenPublicPlayers,
		"openPrivatePlayers":              session.OpenPrivatePlayers,
		"sortWeight":                      session.SortWeight,
		"started":                         session.Started,
		"publicPlayers":                   session.PublicPlayers,
		"privatePlayers":                  session.PrivatePlayers,
	}
}

func getString2(body map[string]interface{}, key string) string {
	if val, ok := body[key].(string); ok {
		return val
	}
	return ""
}

func getInt(body map[string]interface{}, key string) int {
	if val, ok := body[key].(float64); ok {
		return int(val)
	}
	return 0
}

func getBool(body map[string]interface{}, key string) bool {
	if val, ok := body[key].(bool); ok {
		return val
	}
	return false
}
