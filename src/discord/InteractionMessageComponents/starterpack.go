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

func (c *InteractionMessageComponent) HandleStarterPack(targetDiscordID string) {
	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var existingAccount models.Account
	errd := accountsCollection.FindOne(context.Background(), bson.M{"discordid": targetDiscordID}).Decode(&existingAccount)

	if errd != nil {
		sendErrorToWebhook("Account not found")
		return
	}

	additionalData := existingAccount.Additional
	lastClaimDateStr, exists := additionalData["lastClaimDate"].(string)
	fr, exists2 := additionalData["bClaimedStarterPack"].(bool)

	var lastClaimDate time.Time
	if exists {
		lastClaimDate, errd = time.Parse(time.RFC3339, lastClaimDateStr)
		if errd != nil {
			sendErrorToWebhook("Invalid date format for last claim date")
			return
		}
	}

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
		StarterPackFuncs := packs.StarterPack{}
		Items := StarterPackFuncs.GetItems()

		additionalData["bClaimedStarterPack"] = true
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
	}

	if time.Since(lastClaimDate).Hours() > 7*24 || !exists {
		vbucksReward := 2800
		if !exists {
			additionalData["lastClaimDate"] = time.Now().UTC().Format(time.RFC3339)
		} else {
			additionalData["lastClaimDate"] = time.Now().UTC().Format(time.RFC3339)
		}

		profileId := "common_core"
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

		amountFloat := float64(vbucksReward)

		items := profileData["items"].(map[string]interface{})
		mtxPurchased := items["Currency:MtxPurchased"].(map[string]interface{})

		if qty, ok := mtxPurchased["quantity"].(float64); ok {
			mtxPurchased["quantity"] = qty + amountFloat
		} else {
			mtxPurchased["quantity"] = amountFloat
		}

		_, err := accountsCollection.UpdateOne(
			context.Background(),
			bson.M{"accountid": existingAccount.AccountID},
			bson.M{"$set": bson.M{"additional": additionalData}},
		)
		if err != nil {
			sendErrorToWebhook("Failed to update account with reward!")
			return
		}

		_, err = profilesCollection.UpdateOne(context.Background(), bson.M{"accountid": existingAccount.AccountID}, bson.M{
			"$set": bson.M{"profiledata.common_core.items": items},
		})
		if err != nil {
			sendErrorToWebhook("Failed to update profile with reward!")
			return
		}

		sendSuccessToWebhook("User claimed their Weekly StarterPack rewards and received v-bucks!")
	} else {
		sendErrorToWebhook("User has already claimed their reward within the last 7 days.")
	}
}
