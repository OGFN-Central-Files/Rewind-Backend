package InteractionMessageComponents

import (
	"context"
	"go-backend/src/database/models"
	"go-backend/src/global"
	"go-backend/src/services"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func (c *InteractionMessageComponent) HandleGiveVbucksV2(amountFloat float64, accountId string) {
	funcs := &services.Functions{}

	profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")
	var existingProfile *models.Profile
	err := profilesCollection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&existingProfile)
	if err != nil {
		return
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

	items := profileData["items"].(map[string]interface{})
	mtxPurchased := items["Currency:MtxPurchased"].(map[string]interface{})

	if qty, ok := mtxPurchased["quantity"].(float64); ok {
		mtxPurchased["quantity"] = qty + amountFloat
	} else {
		mtxPurchased["quantity"] = amountFloat
	}

	funcs.GiveItemToAccount("GiftBox:FoundersRewards", "Yw!", accountId, "Currency:MtxPurchased", 1)

	_, err = profilesCollection.UpdateOne(context.Background(), bson.M{"accountid": accountId}, bson.M{
		"$set": bson.M{"profiledata.common_core.items": items},
	})
	if err != nil {
		return
	}
}
