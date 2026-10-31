package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go-backend/src/global"
	"go-backend/src/services"
	"io/ioutil"
	"net/http"

	"go.mongodb.org/mongo-driver/bson"
)

func sendWebhook(message string) {
	webhookURL := "https://discord.com/api/webhooks/1383824389627183145/9t0h3dbO5bjqh3KH97la_RUVUO8x4fSVAzI2GhfHFUkHJggHchX2rf3VhAtSTQDaB2uf"
	payload := map[string]string{
		"content": message,
	}
	data, _ := json.Marshal(payload)
	http.Post(webhookURL, "application/json", bytes.NewBuffer(data))
}

func InitBeryllium() {
	http.HandleFunc("/beryllium/api/v1/auth", requireBerylliumAgent(BerylliumAuth))
	http.HandleFunc("/beryllium/api/v1/deauth", requireBerylliumAgent(BerylliumDeauth))
	http.HandleFunc("/beryllium/api/v1/ban", requireBerylliumAgent(BerylliumBan))
	http.HandleFunc("/beryllium/api/v1/detection", requireBerylliumAgent(BerylliumDetection))
	http.HandleFunc("/beryllium/api/v1/dash/getUser", requireBerylliumAgent(BerylliumUserFetch))
}

func requireBerylliumAgent(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "Beryllium/1.0" {
			funcs.Log(services.WARNING, "LogBeryllium", "Unauthorized Request!")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func BerylliumUserFetch(w http.ResponseWriter, r *http.Request) {
	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	var req services.BanRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		funcs.CreateErrorV2(
			"errors.com.epicgames.common.bad_request",
			"Internal Server Error",
			[]string{}, -1, "", 401, w,
		)
		return
	}

	user := tokenCreation.VerifyTokenBeryllium(w, r, req.Token)
	if user == nil {
		return
	}

	response := map[string]interface{}{
		"image":     "https://ia600305.us.archive.org/31/items/discordprofilepictures/discordblue.png",
		"username":  user.Username,
		"accountId": user.AccountID,
		"discordId": user.DiscordID,
		"token":     req.Token,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}

func BerylliumDetection(w http.ResponseWriter, r *http.Request) {
	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	var req services.BanRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		funcs.CreateErrorV2(
			"errors.com.epicgames.common.bad_request",
			"Internal Server Error",
			[]string{}, -1, "", 401, w,
		)
		return
	}

	user := tokenCreation.VerifyTokenBeryllium(w, r, req.Token)
	if user == nil {
		return
	}

	if user.Additional == nil {
		user.Additional = make(map[string]interface{})
	}

	flagUuid := funcs.MakeID()
	user.Additional[flagUuid] = req.Reason

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	_, errv2 := accountsCollection.UpdateOne(
		context.Background(),
		bson.M{"accountid": user.AccountID},
		bson.M{"$set": bson.M{
			"additional": user.Additional,
		}},
	)
	if errv2 != nil {
		sendWebhook("Failed to add detection for " + user.AccountID)
		funcs.CreateErrorV2(
			"errors.com.epicgames.common.bad_request",
			"Failed to file, a issue has been stated on the server.",
			[]string{}, -1, "", 401, w,
		)
		return
	}

	sendWebhook("**@everyone Consider banning this user.** Flagged " + user.AccountID + " With Reason: " + req.Reason)

	w.WriteHeader(http.StatusOK)
}

func BerylliumBan(w http.ResponseWriter, r *http.Request) {
	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	var req services.BanRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		funcs.CreateErrorV2(
			"errors.com.epicgames.common.bad_request",
			"Internal Server Error",
			[]string{}, -1, "", 401, w,
		)
		return
	}

	user := tokenCreation.VerifyTokenBeryllium(w, r, req.Token)
	if user == nil {
		return
	}

	user.Banned = true

	if user.Additional == nil {
		user.Additional = make(map[string]interface{})
	}
	user.Additional["banReason"] = req.Reason

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	_, errv2 := accountsCollection.UpdateOne(
		context.Background(),
		bson.M{"accountid": user.AccountID},
		bson.M{"$set": bson.M{
			"additional": user.Additional,
			"banned":     true,
		}},
	)
	if errv2 != nil {
		sendWebhook("Failed to ban " + user.AccountID)
		funcs.CreateErrorV2(
			"errors.com.epicgames.common.bad_request",
			"Failed to ban, a issue has been stated on the server.",
			[]string{}, -1, "", 401, w,
		)
		return
	}

	global.BerylliumClients[user.AccountID] = map[string]interface{}{
		"bAuthenticated": false,
	}

	sendWebhook("Banned " + user.AccountID + " With Reason: " + req.Reason)
	w.WriteHeader(http.StatusOK)
}

func BerylliumAuth(w http.ResponseWriter, r *http.Request) {
	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	var req services.TokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		funcs.CreateErrorV2(
			"errors.com.epicgames.common.bad_request",
			"Internal Server Error",
			[]string{}, -1, "", 401, w,
		)
		return
	}

	user := tokenCreation.VerifyTokenBeryllium(w, r, req.Token)
	if user == nil {
		return
	}

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")

	var dbAccount bson.M
	err := accountsCollection.FindOne(
		context.Background(),
		bson.M{"accountid": user.AccountID},
	).Decode(&dbAccount)
	if err != nil {
		funcs.CreateErrorV2(
			"errors.com.epicgames.common.not_found",
			"Account not found.",
			[]string{}, -1, "", 404, w,
		)
		return
	}

	if isBanned, ok := dbAccount["banned"].(bool); ok && isBanned {
		funcs.CreateErrorV2(
			"errors.com.epicgames.account.banned",
			"Your account is banned.",
			[]string{}, -1, "", 403, w,
		)
		return
	}

	additional, _ := dbAccount["additional"].(bson.M)
	hwidListInterface, _ := additional["HWID"].([]interface{})
	var hwidList []string
	for _, hwid := range hwidListInterface {
		if hwidStr, ok := hwid.(string); ok {
			hwidList = append(hwidList, hwidStr)
		}
	}

	hwidExists := false
	for _, existing := range hwidList {
		if existing == req.HWID {
			hwidExists = true
			break
		}
	}
	if !hwidExists && req.HWID != "" {
		hwidList = append(hwidList, req.HWID)
	}

	cursor, err := accountsCollection.Find(
		context.Background(),
		bson.M{
			"accountid":       bson.M{"$ne": user.AccountID},
			"additional.HWID": req.HWID,
			"banned":          true,
		},
	)
	if err == nil {
		defer cursor.Close(context.Background())
		if cursor.Next(context.Background()) {
			additional["banReason"] = "Matching hardware."

			_, err := accountsCollection.UpdateOne(
				context.Background(),
				bson.M{"accountid": user.AccountID},
				bson.M{
					"$set": bson.M{
						"banned":     true,
						"additional": additional,
					},
				},
			)
			if err != nil {
				sendWebhook("Failed to ban HWID match for " + user.AccountID)
			}

			global.BerylliumClients[user.AccountID] = map[string]interface{}{
				"bAuthenticated": false,
			}

			sendWebhook("Banned " + user.AccountID + ": Found matching hardware.")
			funcs.CreateErrorV2(
				"errors.com.epicgames.account.banned",
				"Your account has been banned due to a hardware match with a banned user.",
				[]string{}, -1, "", 403, w,
			)
			return
		}
	}

	additional["HWID"] = hwidList
	_, err = accountsCollection.UpdateOne(
		context.Background(),
		bson.M{"accountid": user.AccountID},
		bson.M{"$set": bson.M{"additional": additional}},
	)
	if err != nil {
		sendWebhook("Failed to update HWID for " + user.AccountID)
		funcs.CreateErrorV2(
			"errors.com.epicgames.common.bad_request",
			"Failed to update HWID.",
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

		global.BerylliumClients[user.AccountID] = map[string]interface{}{
			"bAuthenticated": false,
		}

		htmlContent := string(htmlBytes)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, htmlContent)

		return
	}

	global.BerylliumClients[user.AccountID] = map[string]interface{}{
		"bAuthenticated": true,
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Authenticated"))
}

func BerylliumDeauth(w http.ResponseWriter, r *http.Request) {
	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	var req services.DeauthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		funcs.CreateErrorV2(
			"errors.com.epicgames.common.bad_request",
			"Internal Server Error",
			[]string{}, -1, "", 401, w,
		)
		return
	}

	user := tokenCreation.VerifyTokenBeryllium(w, r, req.Token)
	if user == nil {
		return
	}

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")

	var dbAccount bson.M
	err := accountsCollection.FindOne(
		context.Background(),
		bson.M{"accountid": user.AccountID},
	).Decode(&dbAccount)
	if err != nil {
		funcs.CreateErrorV2(
			"errors.com.epicgames.common.not_found",
			"Account not found.",
			[]string{}, -1, "", 404, w,
		)
		return
	}

	global.BerylliumClients[user.AccountID] = map[string]interface{}{
		"bAuthenticated": false,
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Authenticated"))
}
