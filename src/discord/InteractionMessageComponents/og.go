package InteractionMessageComponents

import (
	"context"
	"go-backend/src/database/models"
	"go-backend/src/definitions/packs"
	"go-backend/src/global"
	"go-backend/src/services"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func (c *InteractionMessageComponent) HandleOgDonator(targetDiscordID string) {
	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var existingAccount models.Account
	errd := accountsCollection.FindOne(context.Background(), bson.M{"discordid": targetDiscordID}).Decode(&existingAccount)

	if errd != nil {
		sendErrorToWebhook("Account not found")
		return
	}

	additionalData := existingAccount.Additional
	fr, exists2 := additionalData["bClaimedOG"].(bool)

	profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")
	var existingProfile *models.Profile
	errd = profilesCollection.FindOne(context.Background(), bson.M{"accountid": existingAccount.AccountID}).Decode(&existingProfile)
	if errd != nil {
		sendErrorToWebhook("Profile was not found!")
		return
	}

	if fr {
		return
	}

	if !exists2 {
		OgDonatorFuncs := packs.OgDonator{}
		Items := OgDonatorFuncs.GetItems()

		additionalData["bClaimedOG"] = true
		profileId := "athena"
		profileData, ok := existingProfile.ProfileData[profileId].(map[string]interface{})
		for _, value := range Items {
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

			items := profileData["items"].(map[string]interface{})
			items[value] = map[string]interface{}{
				"templateId":  value,
				"purchasedAt": time.Now().UTC().Format(time.RFC3339),
				"attributes": map[string]interface{}{
					"item_seen": false,
					"variants":  []interface{}{},
				},
				"quantity": 1,
			}

			funcs := services.Functions{}
			funcs.GiveItemToAccount("GiftBox:GB_MakeGoodAthena", "Yw!", existingAccount.AccountID, value, 1)
		}

		_, err := profilesCollection.UpdateOne(context.Background(), bson.M{"accountid": existingAccount.AccountID}, bson.M{
			"$set": bson.M{"profiledata.athena.items": profileData["items"]},
		})
		if err != nil {
			sendErrorToWebhook("Failed to save!")
			return
		}

		_, err = accountsCollection.UpdateOne(
			context.Background(),
			bson.M{"accountid": existingAccount.AccountID},
			bson.M{"$set": bson.M{"additional": additionalData}},
		)
		if err != nil {
			sendErrorToWebhook("Failed to update account with reward!")
			return
		}
	}
}
