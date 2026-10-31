package InteractionMessageComponents

import (
	"bytes"
	"context"
	"encoding/json"
	"go-backend/src/database/models"
	"go-backend/src/global"
	"net/http"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func (c *InteractionMessageComponent) HandleGiveAll(targetDiscordID string) {
	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var existingAccount models.Account
	errd := accountsCollection.FindOne(context.Background(), bson.M{"discordid": targetDiscordID}).Decode(&existingAccount)

	if errd != nil {
		sendErrorToWebhook("Account not found")
		return
	}

	additionalData := existingAccount.Additional
	if _, exists := additionalData["bClaimedFullLockerReward"]; !exists {
		additionalData["bClaimedFullLockerReward"] = true
		existingAccount.Additional = additionalData

		_, errv2 := accountsCollection.UpdateOne(
			context.Background(),
			bson.M{"accountid": existingAccount.AccountID},
			bson.M{"$set": bson.M{"additional": existingAccount.Additional}},
		)
		if errv2 != nil {
			sendErrorToWebhook("Failed to save bClaimedFullLockerReward to account")
			return
		}
	} else {
		return
	}

	jsonFile, err := os.Open("./static/DefaultProfiles/all.json")
	if err != nil {
		sendErrorToWebhook("Failed to load item data file")
		return
	}
	defer jsonFile.Close()

	var jsonData map[string]interface{}
	if err := json.NewDecoder(jsonFile).Decode(&jsonData); err != nil {
		sendErrorToWebhook("Invalid item data format in JSON file")
		return
	}

	newItems, ok := jsonData["items"].(map[string]interface{})
	if !ok {
		sendErrorToWebhook("'items' field missing or invalid in JSON file")
		return
	}

	profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")
	var existingProfile *models.Profile
	err = profilesCollection.FindOne(context.Background(), bson.M{"accountid": existingAccount.AccountID}).Decode(&existingProfile)
	if err != nil {
		sendErrorToWebhook("Profile was not found!")
		return
	}

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

	profileData["items"] = newItems

	_, err = profilesCollection.UpdateOne(
		context.Background(),
		bson.M{"accountid": existingAccount.AccountID},
		bson.M{"$set": bson.M{
			"profiledata.athena.items": profileData["items"],
		}},
	)

	if err != nil {
		sendErrorToWebhook("Failed to save to query!")
		return
	}

	sendSuccessToWebhook("A user claimed their deluxe rewards!")
}

func sendErrorToWebhook(message string) {
	webhookURL := "https://discord.com/api/webhooks/1383824389627183145/9t0h3dbO5bjqh3KH97la_RUVUO8x4fSVAzI2GhfHFUkHJggHchX2rf3VhAtSTQDaB2uf"
	payload := map[string]string{
		"content": message,
	}
	data, _ := json.Marshal(payload)
	http.Post(webhookURL, "application/json", bytes.NewBuffer(data))
}

func sendSuccessToWebhook(message string) {
	webhookURL := "https://discord.com/api/webhooks/1383824389627183145/9t0h3dbO5bjqh3KH97la_RUVUO8x4fSVAzI2GhfHFUkHJggHchX2rf3VhAtSTQDaB2uf"
	payload := map[string]string{
		"content": message,
	}
	data, _ := json.Marshal(payload)
	http.Post(webhookURL, "application/json", bytes.NewBuffer(data))
}
