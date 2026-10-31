package routes

import (
	"context"
	"encoding/json"
	"go-backend/src/global"
	"go-backend/src/services"
	"net/http"
	"os"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
)

func InitEvents() {
	http.HandleFunc("/api/v1/events/Fortnite/download/", DownloadEventsV1)
}

func DownloadEventsV1(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Path
	rawPath = strings.TrimPrefix(rawPath, "/api/v1/events/Fortnite/download/")
	parts := strings.Split(rawPath, "/")

	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenCreation.VerifyToken(w, r)
	if user == nil {
		return
	}

	if len(parts) < 1 || parts[0] == "" {
		funcs.Log(services.ERROR, "Invalid URL structure", rawPath)
		funcs.CreateError(
			"errors.com.epicgames.validation.validation_failed",
			"Invalid URL structure. Expected format: /api/v1/events/Fortnite/download/{accountId}",
			[]string{},
			1040,
			"any",
			"prod",
			"Invalid URL structure",
			"validation_failed",
			w,
		)
		return
	}

	accountId := parts[0]

	filePath := "./static/responses/events.json"
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		funcs.CreateErrorV2(
			"errors.com.epicgames.events.download_failed",
			"Failed to read events data",
			[]string{}, -1, "", 500, w,
		)
		return
	}

	var eventsFile map[string]interface{}
	err = json.Unmarshal(contentBytes, &eventsFile)
	if err != nil {
		funcs.CreateErrorV2(
			"errors.com.epicgames.events.download_failed",
			"Failed to read events data",
			[]string{}, -1, "", 500, w,
		)
		return
	}

	hype := 0
	divFr := 1

	additional := user.Additional
	if _, exists := additional["hype"]; !exists {
		additional["hype"] = 0
		hype = 0
	} else {
		hype = int(additional["hype"].(int32))
	}

	if _, exists := additional["div"]; !exists {
		additional["div"] = 1
		divFr = 1
	} else {
		divFr = int(additional["div"].(int32))
	}

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	_, err = accountsCollection.UpdateOne(
		context.Background(),
		bson.M{"accountid": user.AccountID},
		bson.M{
			"$set": bson.M{
				"additional": additional,
			},
		},
	)
	if err != nil {
		sendWebhook("Failed to update ArenaData for: " + user.AccountID)
	}

	arenaTemplate := map[string]interface{}{
		"accountId": accountId,
		"gameId":    "Fortnite",
		"persistentScores": map[string]interface{}{
			"NormalHype": hype,
		},
		"tokens": []string{},
	}

	divInt := int(divFr)

	for i := 1; i <= divInt; i++ {
		token := "NormalArenaDiv" + strconv.Itoa(i)
		arenaTemplate["tokens"] = append(arenaTemplate["tokens"].([]string), token)
	}

	finalResponse := map[string]interface{}{
		"player":    arenaTemplate,
		"events":    eventsFile["events"],
		"templates": eventsFile["templates"],
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(finalResponse); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}
