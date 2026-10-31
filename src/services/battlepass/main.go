package BattlePassComponent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go-backend/src/database/models"
	XMPP "go-backend/src/definitions/xmpp"
	"go-backend/src/global"
	"go-backend/src/services"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func sendErrorToWebhook(message string) {
	webhookURL := "https://discord.com/api/webhooks/1383824389627183145/9t0h3dbO5bjqh3KH97la_RUVUO8x4fSVAzI2GhfHFUkHJggHchX2rf3VhAtSTQDaB2uf"
	payload := map[string]string{
		"content": message,
	}
	data, _ := json.Marshal(payload)
	http.Post(webhookURL, "application/json", bytes.NewBuffer(data))
}

type BattlePassComponent struct{}

func (bpc *BattlePassComponent) GrantBattlePass(accountId string, season string) error {
	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var account models.Account
	if err := accountsCollection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&account); err != nil {
		return fmt.Errorf("account not found: %v", err)
	}

	claimed := false
	if account.Additional != nil {
		if val, ok := account.Additional["bClaimedPassS15"].(bool); ok {
			claimed = val
		}
	}

	if claimed {
		return fmt.Errorf("already claimed")
	}

	if account.Additional == nil {
		account.Additional = make(map[string]interface{})
	}
	account.Additional["bClaimedPassS15"] = true

	_, err := accountsCollection.UpdateOne(
		context.Background(),
		bson.M{"accountid": accountId},
		bson.M{"$set": bson.M{"additional": account.Additional}},
	)
	if err != nil {
		return fmt.Errorf("failed to save account: %v", err)
	}

	profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")
	var profile models.Profile
	if err := profilesCollection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&profile); err != nil {
		return fmt.Errorf("athena profile not found: %v", err)
	}

	athena, ok := profile.ProfileData["athena"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid athena profile format")
	}

	data, err := ioutil.ReadFile("static/battlepass/season15/season15.json")
	if err != nil {
		sendErrorToWebhook("Failed to load item data file")
		return nil
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		sendErrorToWebhook("Invalid item data format in JSON file")
		return nil
	}

	stats, ok := athena["stats"].(map[string]interface{})
	if !ok {
		stats = make(map[string]interface{})
		athena["stats"] = stats
	}

	attributes, ok := stats["attributes"].(map[string]interface{})
	if !ok {
		attributes = make(map[string]interface{})
		stats["attributes"] = attributes
	}

	attributes["book_purchased"] = true

	seasonNum, err := strconv.Atoi(global.CurrentVersionInfo.CurrentSeason)
	if err != nil {
		log.Printf("invalid season number: %v", err)
		seasonNum = 0
	}
	attributes["season_num"] = seasonNum

	bookLevel := 0.0
	if val, ok := attributes["book_level"].(float64); ok {
		bookLevel = val
	}
	newLevel := bookLevel
	if newLevel > 100 {
		newLevel = 100
	}
	attributes["book_level"] = newLevel

	freeRewards, _ := jsonData["freeRewards"].([]interface{})
	paidRewards, _ := jsonData["paidRewards"].([]interface{})

	for i := 0; i < int(newLevel); i++ {
		if i < len(freeRewards) {
			if reward, ok := freeRewards[i].(map[string]interface{}); ok {
				applyRewards(athena, reward, accountId, true)
			}
		}
		if i < len(paidRewards) {
			if reward, ok := paidRewards[i].(map[string]interface{}); ok {
				applyRewards(athena, reward, accountId, true)
			}
		}
	}

	_, err = profilesCollection.UpdateOne(
		context.Background(),
		bson.M{"accountid": accountId},
		bson.M{"$set": bson.M{"profiledata.athena": athena}},
	)
	if err != nil {
		return fmt.Errorf("failed to update athena profile: %v", err)
	}

	return nil
}

func applyRewards(athena map[string]interface{}, reward map[string]interface{}, accountid string, new bool) {
	items, _ := athena["items"].(map[string]interface{})
	if items == nil {
		items = make(map[string]interface{})
		athena["items"] = items
	}

	giftBox := "GiftBox:gb_battlepass"
	if new {
		giftBox = "GiftBox:gb_battlepasspurchased"
	}

	for item, val := range reward {
		qty := 0
		switch v := val.(type) {
		case float64:
			qty = int(v)
		case int:
			qty = v
		}

		if giftBox == "GiftBox:gb_battlepasspurchased" {
			funcs := services.Functions{}
			funcs.GiveItemToAccount(giftBox, "Yw!", accountid, item, 1)

			giftBox = "GiftBox:gb_battlepass"

			startingSkin := "AthenaCharacter:CID_A_292_Athena_Commando_F_ExoSuit" // starting item once battlepass gets rewarded, Comment this out on seasons below S17.

			profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")
			var existingProfile *models.Profile
			err := profilesCollection.FindOne(context.Background(), bson.M{"accountid": accountid}).Decode(&existingProfile)
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

				items := profileData["items"].(map[string]interface{})
				items[startingSkin] = map[string]interface{}{
					"templateId":  startingSkin,
					"purchasedAt": time.Now().UTC().Format(time.RFC3339),
					"attributes": map[string]interface{}{
						"item_seen": false,
						"variants":  []interface{}{},
					},
					"quantity": 1,
				}
			}

			_, err = profilesCollection.UpdateOne(context.Background(), bson.M{"accountid": accountid}, bson.M{
				"$set": bson.M{"profiledata.athena.items": items},
			})

			funcs.GiveItemToAccount(giftBox, "Yw!", accountid, startingSkin, 1)
		}

		if existingItem, ok := items[item].(map[string]interface{}); ok {
			switch item {
			case "Currency:mtxgiveaway":
				if existingQty, ok := existingItem["quantity"].(float64); ok {
					existingItem["quantity"] = existingQty + float64(qty)
				} else {
					existingItem["quantity"] = float64(qty)
				}
			default:
				existingItem["quantity"] = float64(1)
			}
		} else {
			items[item] = map[string]interface{}{
				"templateId": item,
				"attributes": map[string]interface{}{"item_seen": false},
				"quantity":   float64(qty),
			}

			//funcs := services.Functions{}
			//funcs.GiveItemToAccount(giftBox, "Yw!", accountid, item, 1)
		}
	}
}

func (bpc *BattlePassComponent) GiveXP(accountId string, xpAmount int) error {
	profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")

	var profile models.Profile
	if err := profilesCollection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&profile); err != nil {
		return fmt.Errorf("profile not found: %v", err)
	}

	athena, ok := profile.ProfileData["athena"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid athena profile format")
	}

	stats, _ := athena["stats"].(map[string]interface{})
	if stats == nil {
		stats = make(map[string]interface{})
		athena["stats"] = stats
	}

	attributes, _ := stats["attributes"].(map[string]interface{})
	if attributes == nil {
		attributes = make(map[string]interface{})
		stats["attributes"] = attributes
	}

	data, err := ioutil.ReadFile("static/battlepass/season15/Season15_XP.json")
	if err != nil {
		return fmt.Errorf("failed to read XP data: %v", err)
	}

	var xpData []map[string]interface{}
	if err := json.Unmarshal(data, &xpData); err != nil {
		return fmt.Errorf("invalid XP JSON format: %v", err)
	}

	xpRows, ok := xpData[0]["Rows"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing XP Rows in data")
	}

	currentLevel := toInt(attributes["level"])
	currentXP := toInt(attributes["xp"])
	accountLevel := toInt(attributes["accountLevel"])

	currentXP += xpAmount

	for {
		levelStr := fmt.Sprintf("%d", currentLevel)
		nextXPData, ok := xpRows[levelStr].(map[string]interface{})
		if !ok {
			break
		}
		xpToNext := int(nextXPData["XpToNextLevel"].(float64))
		if currentXP < xpToNext {
			break
		}

		currentXP -= xpToNext
		currentLevel++
		accountLevel++

		if true {
			currentAmount := toInt(attributes["battlestars"])
			attributes["battlestars"] = int32(currentAmount + 5)
		}
	}

	attributes["xp"] = int32(currentXP)
	attributes["level"] = int32(currentLevel)
	attributes["accountLevel"] = int32(accountLevel)
	attributes["book_level"] = int32(currentLevel)
	attributes["book_xp"] = int32(currentXP)

	attributes["season_num"] = 19
	attributes["book_purchased"] = true

	global.GiftReceivedUsers[accountId] = map[string]interface{}{
		"bReceivedGift": true,
	}

	xmpp := XMPP.XmppService{}
	xmpp.SendXmppMessageToClient(accountId, map[string]interface{}{
		"type":      "com.epicgames.gift.received",
		"payload":   map[string]interface{}{},
		"timestamp": time.Now().Unix(),
	})

	_, err = profilesCollection.UpdateOne(
		context.Background(),
		bson.M{"accountid": accountId},
		bson.M{"$set": bson.M{"profiledata.athena": athena}},
	)
	if err != nil {
		return fmt.Errorf("failed to save profile after XP grant: %v", err)
	}

	return nil
}

func toInt(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	default:
		return 0
	}
}
