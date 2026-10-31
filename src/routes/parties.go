package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go-backend/src/database/models"
	XMPP "go-backend/src/definitions/xmpp"
	"go-backend/src/global"
	"go-backend/src/services"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"go.mongodb.org/mongo-driver/bson"
)

type FriendEntry struct {
	AccountID string `bson:"accountId" json:"accountId"`
}

type FriendList struct {
	Accepted []FriendEntry `bson:"accepted" json:"accepted"`
}

type Friends struct {
	AccountID string     `bson:"accountId" json:"accountId"`
	List      FriendList `bson:"list" json:"list"`
}

type Ping struct {
	ID        string                 `json:"id"`
	SentBy    string                 `json:"sent_by"`
	SentTo    string                 `json:"sent_to"`
	SentAt    string                 `json:"sent_at"`
	ExpiresAt string                 `json:"expires_at"`
	Meta      map[string]interface{} `json:"meta"`
}

type Invite struct {
	PartyID   string                 `json:"party_id"`
	SentBy    string                 `json:"sent_by"`
	SentTo    string                 `json:"sent_to"`
	Meta      map[string]interface{} `json:"meta"`
	SentAt    string                 `json:"sent_at"`
	UpdatedAt string                 `json:"updated_at"`
	ExpiresAt string                 `json:"expires_at"`
	Status    string                 `json:"status"`
}

type Connection struct {
	ID              string                 `json:"id"`
	ConnectedAt     string                 `json:"connected_at"`
	UpdatedAt       string                 `json:"updated_at"`
	YieldLeadership bool                   `json:"yield_leadership"`
	Meta            map[string]interface{} `json:"meta"`
}

type Member struct {
	AccountID   string                 `json:"account_id"`
	Meta        map[string]interface{} `json:"meta"`
	Connections []Connection           `json:"connections"`
	Revision    int                    `json:"revision"`
	UpdatedAt   string                 `json:"updated_at"`
	JoinedAt    string                 `json:"joined_at"`
	Role        string                 `json:"role"`
}

type Party struct {
	ID         string                 `json:"id"`
	CreatedAt  string                 `json:"created_at"`
	UpdatedAt  string                 `json:"updated_at"`
	Config     map[string]interface{} `json:"config"`
	Members    []Member               `json:"members"`
	Applicants []interface{}          `json:"applicants"`
	Meta       map[string]interface{} `json:"meta"`
	Invites    []Invite               `json:"invites"`
	Revision   int                    `json:"revision"`
	Intentions []interface{}          `json:"intentions"`
}

type Intention struct {
	RequesterID   string                 `json:"requester_id"`
	RequesterDN   string                 `json:"requester_dn"`
	RequesterPL   string                 `json:"requester_pl"`
	RequesterPLDN string                 `json:"requester_pl_dn"`
	RequesteeID   string                 `json:"requestee_id"`
	Meta          map[string]interface{} `json:"meta"`
	ExpiresAt     string                 `json:"expires_at"`
	SentAt        string                 `json:"sent_at"`
}

type AuthToken struct {
	AccessToken string    `json:"access_token"`
	ExpiresIn   int       `json:"expires_in"`
	TokenType   string    `json:"token_type"`
	App         string    `json:"app"`
	ClientID    string    `json:"client_id"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type VCParticipant struct {
	PUID      string `json:"puid"`
	ClientIP  string `json:"clientIP"`
	HardMuted bool   `json:"hardMuted"`
}

type RoomParticipant struct {
	PUID  string `json:"puid"`
	Token string `json:"token"`
}

type RoomResponse struct {
	RoomID        string            `json:"roomId"`
	ClientBaseURL string            `json:"clientBaseUrl"`
	Participants  []RoomParticipant `json:"participants"`
}

type VCInfo struct {
	Name   string            `json:"name"`
	URL    string            `json:"url"`
	Tokens map[string]string `json:"tokens"`
}

var (
	parties        = make(map[string]Party)
	pings          []Ping
	mutex          sync.RWMutex
	vcParticipants = map[string][]VCParticipant{}
	vcInfo         = map[string]VCInfo{}
	lastClient     *AuthToken
)

func InitParties() {
	http.HandleFunc("/party/api/v1/Fortnite/user/", handleUser)
	http.HandleFunc("/party/api/v1/Fortnite/parties", HandleCreateParty)
	http.HandleFunc("/party/api/v1/Fortnite/parties/", HandlePartyRoutes)
}

func HandlePartyRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPatch {
		segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

		if len(segments) == 9 &&
			segments[0] == "party" &&
			segments[1] == "api" &&
			segments[2] == "v1" &&
			segments[3] == "Fortnite" &&
			segments[4] == "parties" &&
			segments[6] == "members" &&
			segments[8] == "meta" {

			pid := segments[5]
			accountId := segments[7]
			HandleUpdateMemberMeta(w, r, pid, accountId)
			return
		}

		if len(segments) == 6 &&
			segments[0] == "party" &&
			segments[1] == "api" &&
			segments[2] == "v1" &&
			segments[3] == "Fortnite" &&
			segments[4] == "parties" {

			r = muxSetParam(r, "pid", segments[5])
			HandleUpdateParty(w, r)
			return
		}
	}

	if r.Method == http.MethodGet {
		segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

		if len(segments) == 6 &&
			segments[0] == "party" &&
			segments[1] == "api" &&
			segments[2] == "v1" &&
			segments[3] == "Fortnite" &&
			segments[4] == "parties" {

			r = muxSetParam(r, "pid", segments[5])
			HandleGetParty(w, r)
			return
		}

		if len(segments) >= 11 &&
			segments[4] == "user" &&
			segments[6] == "pings" &&
			segments[8] == "parties" {
			HandleGetPingParties(w, r, segments[5], segments[7])
			return
		}
	}

	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if r.Method == http.MethodDelete && len(segments) >= 8 && segments[6] == "members" {
		HandleRemoveMember(w, r, segments[5], segments[7])
		return
	}

	if r.Method == http.MethodPost && len(segments) >= 9 && segments[6] == "members" && segments[8] == "join" {
		HandleJoinPartyMember(w, r, segments[5], segments[7])
		return
	}

	if r.Method == http.MethodPost && len(segments) >= 9 && segments[6] == "members" && segments[8] == "promote" {
		HandlePromotePartyMember(w, r, segments[5], segments[7])
		return
	}

	if r.Method == http.MethodPost && len(segments) >= 9 && segments[4] == "user" && segments[6] == "pings" {
		HandleSendPing(w, r, segments[5], segments[7])
		return
	}

	if r.Method == http.MethodDelete && len(segments) >= 9 && segments[4] == "user" && segments[6] == "pings" {
		HandleDeletePing(w, r, segments[5], segments[7])
		return
	}

	if r.Method == http.MethodPost && len(segments) == 8 &&
		segments[4] == "parties" &&
		segments[6] == "invites" {
		HandleInviteToParty(w, r, segments[5], segments[7])
		return
	}

	if r.Method == http.MethodPost && len(segments) == 9 &&
		segments[4] == "user" &&
		segments[6] == "pings" &&
		segments[8] == "join" {
		HandleJoinViaPing(w, r, segments[5], segments[7])
		return
	}

	if r.Method == http.MethodPost {
		segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

		if len(segments) >= 9 &&
			segments[0] == "party" &&
			segments[1] == "api" &&
			segments[2] == "v1" &&
			segments[3] == "Fortnite" &&
			segments[4] == "parties" &&
			segments[6] == "invites" &&
			(segments[8] == "decline" || (len(segments) > 9 && segments[len(segments)-1] == "decline")) {

			pid := segments[5]
			accountId := segments[7]
			HandleDeclinePartyInvite(w, r, pid, accountId)
			return
		}
	}

	if r.Method == http.MethodPost {
		segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

		if len(segments) == 9 &&
			segments[0] == "party" &&
			segments[1] == "api" &&
			segments[2] == "v1" &&
			segments[3] == "Fortnite" &&
			segments[4] == "members" &&
			segments[6] == "intentions" {

			accountId := segments[5]
			senderId := segments[7]

			HandleSendPartyIntention(w, r, accountId, senderId)
			return
		}
	}

	if r.Method == http.MethodPost {
		segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

		if len(segments) == 10 &&
			segments[0] == "party" &&
			segments[1] == "api" &&
			segments[2] == "v1" &&
			segments[3] == "Fortnite" &&
			segments[4] == "parties" &&
			segments[6] == "members" &&
			segments[8] == "conferences" &&
			segments[9] == "connection" {

			// pid := segments[5]
			// accountId := segments[7]

			HandleVoiceConferenceConnection(w, r)
			return
		}
	}

	http.Error(w, "Not Found", http.StatusNotFound)
}

func handleUser(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	segments := strings.Split(strings.TrimPrefix(path, "/party/api/v1/Fortnite/user/"), "/")
	accountID := segments[0]

	if len(segments) > 1 && segments[1] == "notifications" && len(segments) > 2 && segments[2] == "undelivered" && len(segments) > 3 && segments[3] == "count" {
		handleUndeliveredCount(w, r, accountID)
	} else {
		handleUserData(w, r, accountID)
	}
}

func handleUndeliveredCount(w http.ResponseWriter, r *http.Request, accountID string) {
	mutex.RLock()
	defer mutex.RUnlock()

	var userParty *Party
	for _, p := range parties {
		for _, m := range p.Members {
			if m.AccountID == accountID {
				userParty = &p
				break
			}
		}
		if userParty != nil {
			break
		}
	}

	pingCount := 0
	for _, ping := range pings {
		if ping.ID == accountID {
			pingCount++
		}
	}

	inviteCount := 0
	if userParty != nil {
		for _, invite := range userParty.Invites {
			if invite.SentTo == accountID {
				inviteCount++
			}
		}
	}

	response := map[string]int{
		"pings":   pingCount,
		"invites": inviteCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleUserData(w http.ResponseWriter, r *http.Request, accountID string) {
	mutex.RLock()
	defer mutex.RUnlock()

	var currentParties []Party
	var userPings []Ping

	for _, party := range parties {
		for _, m := range party.Members {
			if m.AccountID == accountID {
				currentParties = append(currentParties, party)
				break
			}
		}
	}

	for _, ping := range pings {
		if ping.ID == accountID {
			userPings = append(userPings, ping)
		}
	}

	response := map[string]interface{}{
		"current": currentParties,
		"pending": []interface{}{},
		"invites": []interface{}{},
		"pings":   userPings,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func HandleCreateParty(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var reqBody struct {
		JoinInfo struct {
			Connection struct {
				ID              string                 `json:"id"`
				Meta            map[string]interface{} `json:"meta"`
				YieldLeadership bool                   `json:"yield_leadership"`
			} `json:"connection"`
			Meta map[string]interface{} `json:"meta"`
		} `json:"join_info"`
		Config map[string]interface{} `json:"config"`
		Meta   map[string]interface{} `json:"meta"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if reqBody.JoinInfo.Connection.ID == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{})
		return
	}

	accountID := strings.Split(reqBody.JoinInfo.Connection.ID, "@prod")[0]

	mutex.Lock()
	defer mutex.Unlock()

	for pid, party := range parties {
		for _, member := range party.Members {
			if member.AccountID == accountID {
				delete(parties, pid)
				break
			}
		}
	}

	id := strings.ReplaceAll(funcs.MakeID(), "-", "")
	now := time.Now().Format(time.RFC3339)

	party := Party{
		ID:        id,
		CreatedAt: now,
		UpdatedAt: now,
		Config:    reqBody.Config,
		Members: []Member{
			{
				AccountID: accountID,
				Meta:      reqBody.JoinInfo.Meta,
				Connections: []Connection{
					{
						ID:              reqBody.JoinInfo.Connection.ID,
						ConnectedAt:     now,
						UpdatedAt:       now,
						YieldLeadership: reqBody.JoinInfo.Connection.YieldLeadership,
						Meta:            reqBody.JoinInfo.Connection.Meta,
					},
				},
				Revision:  0,
				UpdatedAt: now,
				JoinedAt:  now,
				Role:      "CAPTAIN",
			},
		},
		Applicants: []interface{}{},
		Meta:       reqBody.Meta,
		Invites:    []Invite{},
		Revision:   0,
		Intentions: []interface{}{},
	}

	parties[id] = party

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(party)
}

func HandleUpdateParty(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 7 || pathParts[6] == "" {
		http.Error(w, "Party ID not provided", http.StatusBadRequest)
		return
	}
	pid := pathParts[6]

	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenCreation.VerifyToken(w, r)
	if user == nil {
		funcs.Log("Error", "Token verification failed", r.URL.Path)
		return
	}
	accountID := user.AccountID

	mutex.Lock()
	defer mutex.Unlock()

	party, exists := parties[pid]
	if !exists {
		http.Error(w, fmt.Sprintf("Party %s does not exist", pid), http.StatusNotFound)
		return
	}

	var editingMember *Member
	for i := range party.Members {
		if party.Members[i].AccountID == accountID {
			editingMember = &party.Members[i]
			break
		}
	}
	if editingMember == nil {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}
	if editingMember.Role != "CAPTAIN" {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	var reqBody struct {
		Config map[string]interface{} `json:"config"`
		Meta   struct {
			Update map[string]interface{} `json:"update"`
			Delete []string               `json:"delete"`
		} `json:"meta"`
		Revision int `json:"revision"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if reqBody.Config != nil {
		for k, v := range reqBody.Config {
			party.Config[k] = v
		}
	}

	for _, key := range reqBody.Meta.Delete {
		delete(party.Meta, key)
	}

	for k, v := range reqBody.Meta.Update {
		party.Meta[k] = v
	}

	party.Revision = reqBody.Revision
	party.UpdatedAt = time.Now().Format(time.RFC3339)
	parties[pid] = party

	captainID := ""
	for _, m := range party.Members {
		if m.Role == "CAPTAIN" {
			captainID = m.AccountID
			break
		}
	}

	for _, member := range party.Members {
		xmpp := XMPP.XmppService{}
		xmpp.SendXmppMessageToClient(member.AccountID, map[string]interface{}{
			"captain_id":            captainID,
			"created_at":            party.CreatedAt,
			"invite_ttl_seconds":    14400,
			"max_number_of_members": party.Config["max_size"],
			"ns":                    "Fortnite",
			"party_id":              party.ID,
			"party_privacy_type":    party.Config["joinability"],
			"party_state_overriden": map[string]interface{}{},
			"party_state_removed":   reqBody.Meta.Delete,
			"party_state_updated":   reqBody.Meta.Update,
			"party_sub_type":        party.Meta["urn:epic:cfg:party-type-id_s"],
			"party_type":            "DEFAULT",
			"revision":              party.Revision,
			"sent":                  time.Now().Format(time.RFC3339),
			"type":                  "com.epicgames.social.party.notification.v0.PARTY_UPDATED",
			"updated_at":            party.UpdatedAt,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

func HandleUpdateMemberMeta(w http.ResponseWriter, r *http.Request, pid, accountID string) {
	if r.Method != http.MethodPatch {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var reqBody struct {
		Update   map[string]interface{} `json:"update"`
		Delete   json.RawMessage        `json:"delete"`
		Revision int                    `json:"revision"`
	}

	var deleteMap map[string]interface{}
	if len(reqBody.Delete) > 0 && string(reqBody.Delete) != "[]" {
		if err := json.Unmarshal(reqBody.Delete, &deleteMap); err != nil {
			log.Println("Invalid delete format:", err)
			http.Error(w, "Invalid delete format: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	bodyBytes, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		log.Println("Invalid JSON:", err)
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	party, ok := parties[pid]
	if !ok {
		http.Error(w, fmt.Sprintf("Party %s does not exist!", pid), http.StatusNotFound)
		return
	}

	memberIdx := -1
	for i, m := range party.Members {
		if m.AccountID == accountID {
			memberIdx = i
			break
		}
	}

	if memberIdx == -1 {
		http.Error(w, "Member not found", http.StatusNotFound)
		return
	}

	member := party.Members[memberIdx]

	for k := range deleteMap {
		delete(member.Meta, k)
	}

	for k, v := range reqBody.Update {
		member.Meta[k] = v
	}

	member.Revision = reqBody.Revision
	member.UpdatedAt = time.Now().Format(time.RFC3339)
	party.Members[memberIdx] = member
	party.UpdatedAt = time.Now().Format(time.RFC3339)
	parties[pid] = party

	w.WriteHeader(http.StatusNoContent)

	for _, m := range party.Members {
		xmpp := XMPP.XmppService{}
		xmpp.SendXmppMessageToClient(m.AccountID, map[string]interface{}{
			"account_id":              accountID,
			"account_dn":              member.Meta["urn:epic:member:dn_s"],
			"member_state_updated":    reqBody.Update,
			"member_state_removed":    reqBody.Delete,
			"member_state_overridden": map[string]interface{}{},
			"party_id":                party.ID,
			"updated_at":              time.Now().Format(time.RFC3339),
			"sent":                    time.Now().Format(time.RFC3339),
			"revision":                member.Revision,
			"ns":                      "Fortnite",
			"type":                    "com.epicgames.social.party.notification.v0.MEMBER_STATE_UPDATED",
		})
	}
}

func HandleGetParty(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 7 || pathParts[6] == "" {
		http.Error(w, "Party ID not provided", http.StatusBadRequest)
		return
	}
	pid := pathParts[6]

	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenCreation.VerifyToken(w, r)
	if user == nil {
		return
	}

	mutex.RLock()
	party, exists := parties[pid]
	mutex.RUnlock()

	if !exists {
		http.Error(w, fmt.Sprintf("Party %s does not exist", pid), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(party)
}

func HandleRemoveMember(w http.ResponseWriter, r *http.Request, pid string, accountId string) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenCreation.VerifyToken(w, r)
	if user == nil {
		return
	}
	requesterId := user.AccountID

	mutex.Lock()
	defer mutex.Unlock()

	party, exists := parties[pid]
	if !exists {
		http.Error(w, fmt.Sprintf("Party %s does not exist", pid), http.StatusNotFound)
		return
	}

	var mIndex int = -1
	for i, member := range party.Members {
		if member.AccountID == requesterId {
			mIndex = i
			break
		}
	}
	if mIndex == -1 {
		http.Error(w, "Member not found", http.StatusNotFound)
		return
	}
	member := party.Members[mIndex]

	if requesterId != accountId && member.Role != "CAPTAIN" {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	for _, m := range party.Members {
		xmpp := XMPP.XmppService{}
		xmpp.SendXmppMessageToClient(m.AccountID, map[string]interface{}{
			"account_id":          accountId,
			"member_state_update": map[string]interface{}{},
			"ns":                  "Fortnite",
			"party_id":            pid,
			"revision":            party.Revision,
			"sent":                time.Now().Format(time.RFC3339),
			"type":                "com.epicgames.social.party.notification.v0.MEMBER_LEFT",
		})
	}

	party.Members = append(party.Members[:mIndex], party.Members[mIndex+1:]...)

	if len(party.Members) == 0 {
		delete(parties, pid)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var rsaKey string
	if _, ok := party.Meta["Default:RawSquadAssignments_j"]; ok {
		rsaKey = "Default:RawSquadAssignments_j"
	} else if _, ok := party.Meta["RawSquadAssignments_j"]; ok {
		rsaKey = "RawSquadAssignments_j"
	}

	if rsaKey != "" {
		var rsa struct {
			RawSquadAssignments []struct {
				MemberId string `json:"memberId"`
			} `json:"RawSquadAssignments"`
		}
		if err := json.Unmarshal([]byte(party.Meta[rsaKey].(string)), &rsa); err == nil {
			for i, a := range rsa.RawSquadAssignments {
				if a.MemberId == accountId {
					rsa.RawSquadAssignments = append(rsa.RawSquadAssignments[:i], rsa.RawSquadAssignments[i+1:]...)
					break
				}
			}
			if updatedRSA, err := json.Marshal(rsa); err == nil {
				party.Meta[rsaKey] = string(updatedRSA)
			}
		}
	}

	var captain *Member
	for i := range party.Members {
		if party.Members[i].Role == "CAPTAIN" {
			captain = &party.Members[i]
			break
		}
	}
	if captain == nil && len(party.Members) > 0 {
		party.Members[0].Role = "CAPTAIN"
		captain = &party.Members[0]
	}

	party.UpdatedAt = time.Now().Format(time.RFC3339)
	parties[pid] = party

	for _, m := range party.Members {
		xmpp := XMPP.XmppService{}
		xmpp.SendXmppMessageToClient(m.AccountID, map[string]interface{}{
			"captain_id":            captain.AccountID,
			"created_at":            party.CreatedAt,
			"invite_ttl_seconds":    14400,
			"max_number_of_members": party.Config["max_size"],
			"ns":                    "Fortnite",
			"party_id":              party.ID,
			"party_privacy_type":    party.Config["joinability"],
			"party_state_overriden": map[string]interface{}{},
			"party_state_removed":   []string{},
			"party_state_updated": map[string]interface{}{
				rsaKey: party.Meta[rsaKey],
			},
			"party_sub_type": party.Meta["urn:epic:cfg:party-type-id_s"],
			"party_type":     "DEFAULT",
			"revision":       party.Revision,
			"sent":           time.Now().Format(time.RFC3339),
			"type":           "com.epicgames.social.party.notification.v0.PARTY_UPDATED",
			"updated_at":     party.UpdatedAt,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

func HandleJoinPartyMember(w http.ResponseWriter, r *http.Request, pid string, accountId string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenCreation.VerifyToken(w, r)
	if user == nil {
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	party, exists := parties[pid]
	if !exists {
		http.Error(w, fmt.Sprintf("Party %s does not exist", pid), http.StatusNotFound)
		return
	}

	for _, member := range party.Members {
		if member.AccountID == accountId {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":   "JOINED",
				"party_id": party.ID,
			})
			return
		}
	}

	var reqBody struct {
		Meta       map[string]interface{} `json:"meta"`
		Connection struct {
			ID              string                 `json:"id"`
			YieldLeadership bool                   `json:"yield_leadership"`
			Meta            map[string]interface{} `json:"meta"`
		} `json:"connection"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	now := time.Now().Format(time.RFC3339)
	connID := reqBody.Connection.ID
	memID := strings.Split(connID, "@prod")[0]

	member := Member{
		AccountID: memID,
		Meta:      reqBody.Meta,
		Connections: []Connection{
			{
				ID:              connID,
				ConnectedAt:     now,
				UpdatedAt:       now,
				YieldLeadership: reqBody.Connection.YieldLeadership,
				Meta:            reqBody.Connection.Meta,
			},
		},
		Revision:  0,
		UpdatedAt: now,
		JoinedAt:  now,
		Role:      "MEMBER",
	}
	if reqBody.Connection.YieldLeadership {
		member.Role = "CAPTAIN"
	}

	party.Members = append(party.Members, member)
	rsaKey := ""
	if _, ok := party.Meta["Default:RawSquadAssignments_j"]; ok {
		rsaKey = "Default:RawSquadAssignments_j"
	} else if _, ok := party.Meta["RawSquadAssignments_j"]; ok {
		rsaKey = "RawSquadAssignments_j"
	}

	var rsa struct {
		RawSquadAssignments []map[string]interface{} `json:"RawSquadAssignments"`
	}
	if rsaKey != "" {
		raw := party.Meta[rsaKey].(string)
		json.Unmarshal([]byte(raw), &rsa)
		rsa.RawSquadAssignments = append(rsa.RawSquadAssignments, map[string]interface{}{
			"memberId":          memID,
			"absoluteMemberIdx": len(party.Members) - 1,
		})
		if updatedRSA, err := json.Marshal(rsa); err == nil {
			party.Meta[rsaKey] = string(updatedRSA)
			party.Revision++
		}
	}

	party.UpdatedAt = now
	parties[pid] = party

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "JOINED",
		"party_id": party.ID,
	})

	captain := Member{}
	for _, m := range party.Members {
		if m.Role == "CAPTAIN" {
			captain = m
			break
		}
	}

	for _, m := range party.Members {
		xmpp := XMPP.XmppService{}
		xmpp.SendXmppMessageToClient(m.AccountID, map[string]interface{}{
			"account_dn": reqBody.Connection.Meta["urn:epic:member:dn_s"],
			"account_id": memID,
			"connection": map[string]interface{}{
				"id":           connID,
				"connected_at": now,
				"updated_at":   now,
				"meta":         reqBody.Connection.Meta,
			},
			"joined_at":            now,
			"member_state_updated": reqBody.Meta,
			"ns":                   "Fortnite",
			"party_id":             party.ID,
			"revision":             0,
			"sent":                 now,
			"type":                 "com.epicgames.social.party.notification.v0.MEMBER_JOINED",
			"updated_at":           now,
		})

		xmpp.SendXmppMessageToClient(m.AccountID, map[string]interface{}{
			"captain_id":            captain.AccountID,
			"created_at":            party.CreatedAt,
			"invite_ttl_seconds":    14400,
			"max_number_of_members": party.Config["max_size"],
			"ns":                    "Fortnite",
			"party_id":              party.ID,
			"party_privacy_type":    party.Config["joinability"],
			"party_state_overriden": map[string]interface{}{},
			"party_state_removed":   []string{},
			"party_state_updated": map[string]interface{}{
				rsaKey: party.Meta[rsaKey],
			},
			"party_sub_type": party.Meta["urn:epic:cfg:party-type-id_s"],
			"party_type":     "DEFAULT",
			"revision":       party.Revision,
			"sent":           now,
			"type":           "com.epicgames.social.party.notification.v0.PARTY_UPDATED",
			"updated_at":     now,
		})
	}
}

func HandlePromotePartyMember(w http.ResponseWriter, r *http.Request, pid string, accountId string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenCreation.VerifyToken(w, r)
	if user == nil {
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	party, exists := parties[pid]
	if !exists {
		http.Error(w, fmt.Sprintf("Party %s does not exist", pid), http.StatusNotFound)
		return
	}

	var captainIndex, promoteIndex int = -1, -1

	for i, member := range party.Members {
		if member.Role == "CAPTAIN" {
			captainIndex = i
		}
		if member.AccountID == accountId {
			promoteIndex = i
		}
	}

	if captainIndex == -1 || party.Members[captainIndex].AccountID != user.AccountID {
		http.Error(w, fmt.Sprintf("User %s is not allowed to promote member %s!", user.AccountID, accountId), http.StatusForbidden)
		return
	}

	if promoteIndex == -1 {
		http.Error(w, fmt.Sprintf("User %s not found in party %s", accountId, pid), http.StatusNotFound)
		return
	}

	party.Members[captainIndex].Role = "MEMBER"
	party.Members[promoteIndex].Role = "CAPTAIN"

	party.UpdatedAt = time.Now().Format(time.RFC3339)
	parties[pid] = party

	w.WriteHeader(http.StatusNoContent)

	for _, member := range party.Members {
		xmpp := XMPP.XmppService{}
		xmpp.SendXmppMessageToClient(member.AccountID, map[string]interface{}{
			"account_id":          accountId,
			"member_state_update": map[string]interface{}{},
			"ns":                  "Fortnite",
			"party_id":            party.ID,
			"revision":            party.Revision,
			"sent":                time.Now().Format(time.RFC3339),
			"type":                "com.epicgames.social.party.notification.v0.MEMBER_NEW_CAPTAIN",
		})
	}
}

func HandleSendPing(w http.ResponseWriter, r *http.Request, accountId string, pingerId string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	tokenManager := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenManager.VerifyToken(w, r)
	if user == nil {
		return
	}

	versionInfo := funcs.GetVersionInfo(r)

	mutex.Lock()
	defer mutex.Unlock()

	for i := 0; i < len(pings); i++ {
		if pings[i].SentTo == accountId && pings[i].SentBy == pingerId {
			pings = append(pings[:i], pings[i+1:]...)
			break
		}
	}

	var body struct {
		Meta map[string]interface{} `json:"meta"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	sentAt := time.Now().UTC()
	expiresAt := sentAt.Add(1 * time.Hour)

	ping := Ping{
		SentBy:    pingerId,
		SentTo:    accountId,
		SentAt:    sentAt.Format(time.RFC3339),
		ExpiresAt: expiresAt.Format(time.RFC3339),
		Meta:      body.Meta,
	}
	pings = append(pings, ping)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ping)

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var existingAccount models.Account
	err := accountsCollection.FindOne(context.Background(), bson.M{"accountid": pingerId}).Decode(&existingAccount)

	if err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	username := existingAccount.Username

	xmpp := XMPP.XmppService{}
	xmpp.SendXmppMessageToClient(accountId, map[string]interface{}{
		"expires":   ping.ExpiresAt,
		"meta":      body.Meta,
		"ns":        "Fortnite",
		"pinger_dn": username,
		"pinger_id": pingerId,
		"sent":      ping.SentAt,
		"version":   fmt.Sprintf("%-5s", strconv.Itoa(funcs.ToInt(versionInfo.Build))),
		"type":      "com.epicgames.social.party.notification.v0.PING",
	})
}

func HandleDeletePing(w http.ResponseWriter, r *http.Request, accountId string, pingerId string) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	tokenManager := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenManager.VerifyToken(w, r)
	if user == nil {
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	for i := 0; i < len(pings); i++ {
		if pings[i].SentTo == accountId && pings[i].SentBy == pingerId {
			pings = append(pings[:i], pings[i+1:]...)
			break
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func HandleGetPingParties(w http.ResponseWriter, r *http.Request, accountId, pingerId string) {

	mutex.Lock()
	defer mutex.Unlock()

	var query []Ping
	for _, p := range pings {
		if p.SentTo == accountId && p.SentBy == pingerId {
			query = append(query, p)
		}
	}
	if len(query) == 0 {
		query = append(query, Ping{SentBy: pingerId})
	}

	var result []map[string]interface{}
	for _, y := range query {
		for _, party := range parties {
			for _, m := range party.Members {
				if m.AccountID == y.SentBy {
					result = append(result, map[string]interface{}{
						"id":         party.ID,
						"created_at": party.CreatedAt,
						"updated_at": party.UpdatedAt,
						"config":     party.Config,
						"members":    party.Members,
						"applicants": []interface{}{},
						"meta":       party.Meta,
						"invites":    []interface{}{},
						"revision":   party.Revision,
					})
					break
				}
			}
		}
	}

	json.NewEncoder(w).Encode(result)
}

func HandleJoinViaPing(w http.ResponseWriter, r *http.Request, accountId, pingerId string) {
	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenCreation.VerifyToken(w, r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	var foundPing *Ping
	for _, p := range pings {
		if p.SentTo == accountId && p.SentBy == pingerId {
			foundPing = &p
			break
		}
	}
	if foundPing == nil {
		foundPing = &Ping{SentBy: pingerId}
	}

	var party *Party
	for id, p := range parties {
		for _, m := range p.Members {
			if m.AccountID == foundPing.SentBy {
				p.ID = id
				party = &p
				break
			}
		}
		if party != nil {
			break
		}
	}
	if party == nil {
		http.Error(w, "Party not found", http.StatusNotFound)
		return
	}

	for _, m := range party.Members {
		if m.AccountID == accountId {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":   "JOINED",
				"party_id": party.ID,
			})
			return
		}
	}

	var req struct {
		Connection struct {
			ID              string                 `json:"id"`
			YieldLeadership bool                   `json:"yield_leadership"`
			Meta            map[string]interface{} `json:"meta"`
		} `json:"connection"`
		Meta map[string]interface{} `json:"meta"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	now := time.Now().Format(time.RFC3339)
	memberID := strings.Split(req.Connection.ID, "@prod")[0]

	member := Member{
		AccountID: memberID,
		Meta:      req.Meta,
		Connections: []Connection{
			{
				ID:              req.Connection.ID,
				ConnectedAt:     now,
				UpdatedAt:       now,
				YieldLeadership: req.Connection.YieldLeadership,
				Meta:            req.Connection.Meta,
			},
		},
		Revision:  0,
		UpdatedAt: now,
		JoinedAt:  now,
		Role:      "MEMBER",
	}

	if req.Connection.YieldLeadership {
		member.Role = "CAPTAIN"
	}
	party.Members = append(party.Members, member)

	var rsaKey string
	if _, ok := party.Meta["Default:RawSquadAssignments_j"]; ok {
		rsaKey = "Default:RawSquadAssignments_j"
	} else {
		rsaKey = "RawSquadAssignments_j"
	}

	if raw, ok := party.Meta[rsaKey]; ok {
		var rsa struct {
			RawSquadAssignments []map[string]interface{} `json:"RawSquadAssignments"`
		}
		_ = json.Unmarshal([]byte(raw.(string)), &rsa)
		rsa.RawSquadAssignments = append(rsa.RawSquadAssignments, map[string]interface{}{
			"memberId":          memberID,
			"absoluteMemberIdx": len(party.Members) - 1,
		})
		newRaw, _ := json.Marshal(rsa)
		party.Meta[rsaKey] = string(newRaw)
		party.Revision++
	}

	party.UpdatedAt = now
	parties[party.ID] = *party

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "JOINED",
		"party_id": party.ID,
	})

	var captainID string
	for _, m := range party.Members {
		if m.Role == "CAPTAIN" {
			captainID = m.AccountID
			break
		}
	}

	xmpp := XMPP.XmppService{}
	for _, m := range party.Members {
		go func(target Member) {
			xmpp.SendXmppMessageToClient(target.AccountID, map[string]interface{}{
				"account_dn": req.Connection.Meta["urn:epic:member:dn_s"],
				"account_id": memberID,
				"connection": map[string]interface{}{
					"connected_at": now,
					"id":           req.Connection.ID,
					"meta":         req.Connection.Meta,
					"updated_at":   now,
				},
				"joined_at":            now,
				"member_state_updated": req.Meta,
				"ns":                   "Fortnite",
				"party_id":             party.ID,
				"revision":             0,
				"sent":                 now,
				"type":                 "com.epicgames.social.party.notification.v0.MEMBER_JOINED",
				"updated_at":           now,
			})

			xmpp.SendXmppMessageToClient(target.AccountID, map[string]interface{}{
				"captain_id":            captainID,
				"created_at":            party.CreatedAt,
				"invite_ttl_seconds":    14400,
				"max_number_of_members": party.Config["max_size"],
				"ns":                    "Fortnite",
				"party_id":              party.ID,
				"party_privacy_type":    party.Config["joinability"],
				"party_state_overriden": map[string]interface{}{},
				"party_state_removed":   []interface{}{},
				"party_state_updated": map[string]interface{}{
					rsaKey: party.Meta[rsaKey],
				},
				"party_sub_type": party.Meta["urn:epic:cfg:party-type-id_s"],
				"party_type":     "DEFAULT",
				"revision":       party.Revision,
				"sent":           now,
				"type":           "com.epicgames.social.party.notification.v0.PARTY_UPDATED",
				"updated_at":     now,
			})
		}(m)
	}
}

func HandleInviteToParty(w http.ResponseWriter, r *http.Request, pid, accountId string) {
	tokenManager := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenManager.VerifyToken(w, r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	memory := funcs.GetVersionInfo(r)

	party, ok := parties[pid]
	if !ok {
		http.Error(w, "Invalid JSON: ", http.StatusBadRequest)
		return
	}

	newInvites := []Invite{}
	for _, inv := range party.Invites {
		if !(inv.SentTo == accountId && inv.SentBy == user.AccountID) {
			newInvites = append(newInvites, inv)
		}
	}
	party.Invites = newInvites

	now := time.Now()
	expires := now.Add(1 * time.Hour)
	var inviteMeta map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&inviteMeta); err != nil {
		http.Error(w, "Invalid invite body", http.StatusBadRequest)
		return
	}

	invite := Invite{
		PartyID:   party.ID,
		SentBy:    user.AccountID,
		SentTo:    accountId,
		Meta:      inviteMeta,
		SentAt:    now.Format(time.RFC3339),
		UpdatedAt: now.Format(time.RFC3339),
		ExpiresAt: expires.Format(time.RFC3339),
		Status:    "SENT",
	}
	party.Invites = append(party.Invites, invite)
	party.UpdatedAt = now.Format(time.RFC3339)
	parties[pid] = party

	var inviter *Member
	for _, m := range party.Members {
		if m.AccountID == user.AccountID {
			inviter = &m
			break
		}
	}

	friends := []string{}

	for _, f := range getList(user.Additional, "friends") {
		if accountId, ok := f["accountId"].(string); ok {
			friends = append(friends, accountId)
		}
	}

	xmpp := XMPP.XmppService{}
	xmpp.SendXmppMessageToClient(accountId, map[string]interface{}{
		"expires":       invite.ExpiresAt,
		"meta":          inviteMeta,
		"ns":            "Fortnite",
		"party_id":      party.ID,
		"inviter_dn":    inviter.Meta["urn:epic:member:dn_s"],
		"inviter_id":    user.AccountID,
		"invitee_id":    accountId,
		"members_count": len(party.Members),
		"sent_at":       invite.SentAt,
		"updated_at":    invite.UpdatedAt,
		"friends_ids":   friends,
		"sent":          now.Format(time.RFC3339),
		"type":          "com.epicgames.social.party.notification.v0.INITIAL_INVITE",
	})

	if r.URL.Query().Get("sendPing") == "true" {
		newPings := []Ping{}
		for _, p := range pings {
			if !(p.SentTo == accountId && p.SentBy == user.AccountID) {
				newPings = append(newPings, p)
			}
		}
		pings = newPings

		ping := Ping{
			SentBy:    user.AccountID,
			SentTo:    accountId,
			SentAt:    now.Format(time.RFC3339),
			ExpiresAt: expires.Format(time.RFC3339),
			Meta:      inviteMeta,
		}
		pings = append(pings, ping)

		xmpp.SendXmppMessageToClient(accountId, map[string]interface{}{
			"expires":   invite.ExpiresAt,
			"meta":      inviteMeta["meta"],
			"ns":        "Fortnite",
			"pinger_dn": inviter.Meta["urn:epic:member:dn_s"],
			"pinger_id": user.AccountID,
			"sent":      invite.SentAt,
			"version":   fmt.Sprintf("%05s", memory.Build),
			"type":      "com.epicgames.social.party.notification.v0.PING",
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

func HandleDeclinePartyInvite(w http.ResponseWriter, r *http.Request, pid string, accountId string) {
	mutex.Lock()
	defer mutex.Unlock()

	newp, ok := parties[pid]
	if !ok {
		http.Error(w, fmt.Sprintf(`{"errorCode":"errors.com.epicgames.party.not_found","message":"Party %s does not exist!"}`, pid), http.StatusNotFound)
		return
	}

	var invite *Invite
	for i := range newp.Invites {
		if newp.Invites[i].SentTo == accountId {
			invite = &newp.Invites[i]
			break
		}
	}

	if invite == nil {
		http.Error(w, fmt.Sprintf(`{"errorCode":"errors.com.epicgames.party.not_found","message":"Invite for %s does not exist!"}`, accountId), http.StatusNotFound)
		return
	}

	var inviter Member
	for _, m := range newp.Members {
		if m.AccountID == invite.SentBy {
			inviter = m
			break
		}
	}

	w.WriteHeader(http.StatusNoContent)

	if inviter.AccountID != "" {
		body := make(map[string]interface{})
		json.NewDecoder(r.Body).Decode(&body)

		xmpp := XMPP.XmppService{}
		xmpp.SendXmppMessageToClient(invite.SentBy, map[string]interface{}{
			"expires":    invite.ExpiresAt,
			"meta":       body,
			"ns":         "Fortnite",
			"party_id":   newp.ID,
			"inviter_dn": inviter.Meta["urn:epic:member:dn_s"],
			"inviter_id": invite.SentBy,
			"invitee_id": accountId,
			"sent_at":    invite.SentAt,
			"updated_at": invite.UpdatedAt,
			"sent":       time.Now().Format(time.RFC3339),
			"type":       "com.epicgames.social.party.notification.v0.INVITE_CANCELLED",
		})
	}
}

func HandleSendPartyIntention(w http.ResponseWriter, r *http.Request, accountId string, senderId string) {
	mutex.Lock()
	defer mutex.Unlock()

	var party *Party

	for _, p := range parties {
		for _, m := range p.Members {
			if m.AccountID == senderId {
				pCopy := p
				party = &pCopy
				break
			}
		}
		if party != nil {
			break
		}
	}

	if party == nil {
		http.Error(w, `{"errorCode":"errors.com.epicgames.party.not_found","message":"Party does not exist!"}`, http.StatusNotFound)
		return
	}

	sender := Member{}
	captain := Member{}
	for _, m := range party.Members {
		if m.AccountID == senderId {
			sender = m
		}
		if m.Role == "CAPTAIN" {
			captain = m
		}
	}

	collection := global.MongoDBClient.Database("fortnite").Collection("friends")
	var friends Friends
	err := collection.FindOne(context.Background(), bson.M{"accountId": accountId}).Decode(&friends)
	if err != nil {
		log.Println("Error fetching friends for", accountId, ":", err)
	}

	var body map[string]interface{}
	json.NewDecoder(r.Body).Decode(&body)

	exp := time.Now().Add(time.Hour).Format(time.RFC3339)
	sent := time.Now().Format(time.RFC3339)

	intention := Intention{
		RequesterID:   senderId,
		RequesterDN:   fmt.Sprintf("%v", sender.Meta["urn:epic:member:dn_s"]),
		RequesterPL:   captain.AccountID,
		RequesterPLDN: fmt.Sprintf("%v", captain.Meta["urn:epic:member:dn_s"]),
		RequesteeID:   accountId,
		Meta:          body,
		ExpiresAt:     exp,
		SentAt:        sent,
	}

	party.Intentions = append(party.Intentions, intention)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(intention)

	friendIDs := make(map[string]bool)
	for _, f := range friends.List.Accepted {
		friendIDs[f.AccountID] = true
	}

	friendPartyMembers := []string{}
	for _, m := range party.Members {
		if friendIDs[m.AccountID] {
			friendPartyMembers = append(friendPartyMembers, m.AccountID)
		}
	}

	xmpp := XMPP.XmppService{}
	xmpp.SendXmppMessageToClient(accountId, map[string]interface{}{
		"expires_at":      exp,
		"requester_id":    senderId,
		"requester_dn":    sender.Meta["urn:epic:member:dn_s"],
		"requester_pl":    captain.AccountID,
		"requester_pl_dn": captain.Meta["urn:epic:member:dn_s"],
		"requestee_id":    accountId,
		"meta":            body,
		"sent_at":         sent,
		"updated_at":      time.Now().Format(time.RFC3339),
		"friends_ids":     friendPartyMembers,
		"members_count":   len(party.Members),
		"party_id":        party.ID,
		"ns":              "Fortnite",
		"sent":            time.Now().Format(time.RFC3339),
		"type":            "com.epicgames.social.party.notification.v0.INITIAL_INTENTION",
	})
}

func HandleVoiceConferenceConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(segments) < 10 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	pid := segments[5]
	accountId := segments[7]

	var body struct {
		Providers struct {
			Vivox bool `json:"vivox"`
			RTCP  bool `json:"rtcp"`
		} `json:"providers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	var responseProviders = make(map[string]interface{})

	if body.Providers.RTCP {
		if lastClient == nil || lastClient.ExpiresAt.Before(time.Now()) {
			form := url.Values{}
			form.Add("grant_type", "client_credentials")
			form.Add("deployment_id", "8949d22a1748462abe3e938fd7e19e5c")

			req, _ := http.NewRequest("POST", "https://api.epicgames.dev/auth/v1/oauth/token", strings.NewReader(form.Encode()))
			req.SetBasicAuth("xyza7891InagL5qaPE6DzJBNn14sBWgF", "NvJM0I6mKNEMCnn89DtEJTiZRfWW6YsAZjicl6IVols")
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				http.Error(w, "Failed to fetch OAuth token", http.StatusInternalServerError)
				return
			}
			defer resp.Body.Close()

			var token AuthToken
			json.NewDecoder(resp.Body).Decode(&token)
			lastClient = &token
		}

		if vcParticipants[pid] == nil {
			vcParticipants[pid] = []VCParticipant{}
		}

		found := false
		for _, p := range vcParticipants[pid] {
			if p.PUID == accountId {
				found = true
				break
			}
		}
		if !found {
			vcParticipants[pid] = append(vcParticipants[pid], VCParticipant{
				PUID:      accountId,
				ClientIP:  r.RemoteAddr,
				HardMuted: false,
			})
		}

		roomReq := map[string]interface{}{
			"participants": vcParticipants[pid],
		}
		reqBody, _ := json.Marshal(roomReq)
		req, _ := http.NewRequest("POST", fmt.Sprintf("https://api.epicgames.dev/rtc/v1/8949d22a1748462abe3e938fd7e19e5c/room/%s", pid), bytes.NewReader(reqBody))
		req.Header.Set("Authorization", fmt.Sprintf("%s %s", strings.Title(lastClient.TokenType), lastClient.AccessToken))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, "Failed to create RTC room", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		var roomResp RoomResponse
		json.NewDecoder(resp.Body).Decode(&roomResp)

		tokens := map[string]string{}
		for _, participant := range roomResp.Participants {
			tokens[participant.PUID] = participant.Token
		}

		vcInfo[pid] = VCInfo{
			Name:   roomResp.RoomID,
			URL:    roomResp.ClientBaseURL,
			Tokens: tokens,
		}

		responseProviders["rtcp"] = map[string]interface{}{
			"participant_token": tokens[accountId],
			"client_base_url":   roomResp.ClientBaseURL,
			"room_name":         roomResp.RoomID,
		}
	} else {
		channelURI := fmt.Sprintf("sip:confctl-g-epicgames.p-%s@mtu1xp.vivox.com", pid)
		userURI := fmt.Sprintf("sip:.epicgames.%s.@mtu1xp.vivox.com", accountId)

		exp := time.Now().Add(2 * time.Hour).Unix()
		claims := jwt.MapClaims{
			"iss": "epicgames",
			"sub": accountId,
			"exp": exp,
			"vxa": "join",
			"f":   userURI,
			"t":   channelURI,
		}

		token, err := GenerateVXToken("zcETsPpEAysznTyDXK4TEzwLQPcTvTAO", claims)
		if err != nil {
			http.Error(w, "Failed to generate Vivox token", http.StatusInternalServerError)
			return
		}

		responseProviders["vivox"] = map[string]string{
			"authorization_token": token,
			"channel_uri":         channelURI,
			"user_uri":            userURI,
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"providers": responseProviders,
	})
}

func muxSetParam(r *http.Request, key, value string) *http.Request {
	ctx := context.WithValue(r.Context(), key, value)
	return r.WithContext(ctx)
}

func muxGetParam(r *http.Request, key string) string {
	val := r.Context().Value(key)
	if val != nil {
		return val.(string)
	}
	return ""
}

func GenerateVXToken(secret string, claims jwt.MapClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}
	return signed, nil
}
