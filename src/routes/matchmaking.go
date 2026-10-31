package routes

import (
	"encoding/json"
	"fmt"
	"go-backend/src/global"
	"go-backend/src/services"
	"io/ioutil"
	"net/http"

	"go.mongodb.org/mongo-driver/bson"
)

func InitMatchmaking() {
	http.HandleFunc("/fortnite/api/game/v2/matchmakingservice/ticket/player/", MatchmakingTicketHandler)
	http.HandleFunc("/fortnite/api/game/v2/matchmaking/account/", proxyRequestMMS)
	http.HandleFunc("/fortnite/api/matchmaking/session/matchMakingRequest", proxyRequestMMS)

	if global.MatchmakingSettings["bEnableFindPlayerEndpoint"].(bool) {
		http.HandleFunc("/fortnite/api/matchmaking/session/findPlayer/", proxyRequestMMS)
	}

	if global.MatchmakingSettings["bHandleExternalJoinRequest"].(bool) {
		http.HandleFunc("/fortnite/api/matchmaking/session/", proxyRequestMMS)
	}

	http.HandleFunc("/fortnite/api/matchmaking/session/", JoinReq)
}

func MatchmakingTicketHandler(w http.ResponseWriter, r *http.Request) {
	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenCreation.VerifyToken(w, r)
	if user == nil {
		funcs.CreateErrorV2(
			"errors.com.epicgames.common.unauthorized",
			"User is not authenticated with Beryllium.",
			[]string{}, -1, "", 401, w,
		)
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

	versionInfo := funcs.GetVersionInfo(r)

	if versionInfo.Build < 24.0 || versionInfo.Build >= 25.0 {
		auth, ok := global.BerylliumClients[user.AccountID]
		if !ok || auth.(map[string]interface{})["bAuthenticated"] != true {
			if global.MatchmakingSettings["bMustHaveBerylliumConnected"] == true {
				funcs.CreateErrorV2(
					"errors.com.epicgames.common.unauthorized",
					"User is not authenticated with Beryllium.",
					[]string{}, -1, "", 401, w,
				)
				return
			}
		}

		if global.MatchmakingSettings["bMustHaveHwidVerified"].(bool) {
			rawHWIDs, ok := user.Additional["HWID"]
			if !ok || rawHWIDs == nil {
				funcs.Log(services.INFO, "LogBeryllium", "No HWID, Denying MMS Handshake.")
				funcs.CreateErrorV2(
					"errors.com.epicgames.common.unauthorized",
					"HWID's not verified with beryllium",
					[]string{}, -1, "", 401, w,
				)
				return
			}

			var hwidList []interface{}
			if bsonArray, ok := rawHWIDs.(bson.A); ok {
				hwidList = bsonArray
			} else if list, ok := rawHWIDs.([]interface{}); ok {
				hwidList = list
			} else {
				funcs.Log(services.INFO, "LogBeryllium", "HWID list is not an array")
				funcs.CreateErrorV2(
					"errors.com.epicgames.common.invaliddata",
					"HWID list is not an array",
					[]string{}, -1, "", 400, w,
				)
				return
			}
			hwidCount := len(hwidList)
			if hwidCount <= 0 || hwidCount > 5 {
				funcs.Log(services.INFO, "LogBeryllium", fmt.Sprintf("Invalid HWID list count: %d", hwidCount))
				funcs.CreateErrorV2(
					"errors.com.epicgames.common.invaliddata",
					"You have been matchmaking banned or your account was flagged.",
					[]string{}, -1, "", 400, w,
				)
				return
			}
		}
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

	proxyRequestMMS(w, r)
}

func JoinReq(w http.ResponseWriter, r *http.Request) {
	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenCreation.VerifyToken(w, r)
	if user == nil {
		return
	}

	response := make(map[string]interface{})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
