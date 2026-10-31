package operations

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"go-backend/src/database/models"
	XMPP "go-backend/src/definitions/xmpp"
	"go-backend/src/global"
	"go-backend/src/services"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Offer struct {
	TemplateId string `json:"templateId"`
	Quantity   int    `json:"quantity"`
	Cost       int    `json:"cost"`
}

func (f *Operations) ExchangeGameCurrencyForBattlePassOffer(w http.ResponseWriter, r *http.Request) {
	type RequestBody struct {
		OfferItemIdList []string `json:"offerItemIdList"`
	}

	rawPath := strings.TrimPrefix(r.URL.Path, "/fortnite/api/game/v2/")
	parts := strings.Split(rawPath, "/")
	funcs := &services.Functions{}

	if len(parts) < 4 {
		funcs.CreateError("errors.com.epicgames.validation.validation_failed", "Invalid URL structure", nil, 1040, "any", "prod", "Invalid URL structure", "validation_failed", w)
		return
	}

	accountId := parts[1]
	profileId := r.URL.Query().Get("profileId")

	var reqBody RequestBody
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil || len(reqBody.OfferItemIdList) == 0 {
		funcs.CreateError("errors.com.epicgames.validation.validation_failed", "Invalid request body", nil, 1040, "any", "prod", "Invalid request", "validation_failed", w)
		return
	}

	if len(reqBody.OfferItemIdList) > 1 {
		funcs.CreateError("errors.com.epicgames.validation.validation_failed", "You can currently not purchase more than 1 item at a time.", []string{"offerItemIdList"}, 1040, "any", "prod", "Multiple items not supported", "validation_failed", w)
		return
	}

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var account *models.Account
	if err := accountsCollection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&account); err != nil {
		funcs.CreateError("errors.com.epicgames.common.not_found", "Account not found", nil, 1004, "any", "prod", "Not found", "not_found", w)
		return
	}

	profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")
	var dbProfile *models.Profile
	if err := profilesCollection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&dbProfile); err != nil {
		funcs.CreateError("errors.com.epicgames.common.not_found", "Profile not found", nil, 1004, "any", "prod", "Not found", "not_found", w)
		return
	}

	profileData := dbProfile.ProfileData[profileId].(map[string]interface{})
	stats := profileData["stats"].(map[string]interface{})["attributes"].(map[string]interface{})

	offerId := reqBody.OfferItemIdList[0]
	seasonOffers := LoadOffers("static/battlepass/Season19Offers.json")
	offer, found := seasonOffers[offerId]
	if !found {
		funcs.CreateError("errors.com.epicgames.fortnite.id_invalid", "Offer not found", []string{offerId}, 16027, "any", "prod", "Invalid offer ID", "id_invalid", w)
		return
	}

	battlestars := toInt(stats["battlestars"])
	if battlestars < offer.Cost {
		funcs.CreateError("errors.com.epicgames.validation.validation_failed", "Not enough battlestars", []string{"battlestars"}, 1040, "any", "prod", "Insufficient battlestars", "validation_failed", w)
		return
	}

	stats["battlestars"] = battlestars - offer.Cost
	stats["battlestars_season_total"] = toInt(stats["battlestars_season_total"]) - offer.Cost

	if _, ok := stats["purchased_bp_offers"]; !ok {
		stats["purchased_bp_offers"] = []interface{}{}
	}
	var purchasedOffers primitive.A
	if val, ok := stats["purchased_bp_offers"]; ok {
		if arr, ok := val.(primitive.A); ok {
			purchasedOffers = arr
		} else {
			purchasedOffers = primitive.A{}
		}
	} else {
		purchasedOffers = primitive.A{}
	}

	purchaseEntry := map[string]interface{}{
		"offerId":           offerId,
		"bIsFreePassReward": false,
		"purchaseDate":      time.Now().Format(time.RFC3339),
		"lootResult": []interface{}{map[string]interface{}{
			"itemType":    offer.TemplateId,
			"itemGuid":    offer.TemplateId,
			"itemProfile": profileId,
			"quantity":    offer.Quantity,
		}},
		"currencyType":      "battlestars",
		"totalCurrencyPaid": offer.Cost,
	}

	if offer.TemplateId == "Currency:mtxgiveaway" {
		common_core := dbProfile.ProfileData["common_core"].(map[string]interface{})
		items := common_core["items"].(map[string]interface{})
		mtxPurchased := items["Currency:MtxPurchased"].(map[string]interface{})

		if qty, ok := mtxPurchased["quantity"].(float64); ok {
			mtxPurchased["quantity"] = qty + float64(offer.Quantity)
		} else {
			mtxPurchased["quantity"] = offer.Quantity
		}

		profilesCollection.UpdateOne(context.Background(), bson.M{"accountid": dbProfile.AccountID}, bson.M{
			"$set": bson.M{"profiledata.common_core.items": items},
		})
	} else {
		items := profileData["items"].(map[string]interface{})
		items[offer.TemplateId] = map[string]interface{}{
			"templateId":  offer.TemplateId,
			"purchasedAt": time.Now().UTC().Format(time.RFC3339),
			"attributes": map[string]interface{}{
				"item_seen": false,
				"variants":  []interface{}{},
			},
			"quantity": offer.Quantity,
		}
	}

	purchasedOffers = append(purchasedOffers, purchaseEntry)
	stats["purchased_bp_offers"] = purchasedOffers

	profileChanges := []interface{}{
		map[string]interface{}{
			"changeType": "statModified",
			"name":       "battlestars",
			"value":      stats["battlestars"],
		},
		map[string]interface{}{
			"changeType": "statModified",
			"name":       "battlestars_season_total",
			"value":      stats["battlestars_season_total"],
		},
		map[string]interface{}{
			"changeType": "statModified",
			"name":       "purchased_bp_offers",
			"value":      purchasedOffers,
		},
		map[string]interface{}{
			"changeType": "itemAdded",
			"itemId":     offer.TemplateId,
			"item": map[string]interface{}{
				"templateId": offer.TemplateId,
				"attributes": map[string]interface{}{
					"item_seen": false,
					"variants":  []interface{}{},
				},
				"quantity": offer.Quantity,
			},
		},
	}

	profileRev, _ := funcs.GetRVN(profileData, "rvn")
	profilerev2, _ := funcs.GetAndMultiplyRVN(profileData, "rvn", 1)
	profilecommandrev2, _ := funcs.GetAndMultiplyRVN(profileData, "commandRevision", 1)

	_, err := profilesCollection.UpdateOne(
		context.Background(),
		bson.M{"accountid": accountId},
		bson.M{"$set": bson.M{"profiledata." + profileId: profileData}},
	)
	if err != nil {
		funcs.Log(services.ERROR, "Failed to update profile", err.Error())
		http.Error(w, "Failed to update profile", http.StatusInternalServerError)
		return
	}

	global.GiftReceivedUsers[accountId] = map[string]interface{}{
		"bReceivedGift": true,
	}

	xmpp := XMPP.XmppService{}
	xmpp.SendXmppMessageToClient(accountId, map[string]interface{}{
		"type":      "com.epicgames.gift.received",
		"payload":   map[string]interface{}{},
		"timestamp": time.Now().Unix(),
	})

	response := map[string]interface{}{
		"profileRevision":            profilerev2,
		"profileId":                  profileId,
		"profileChangesBaseRevision": profileRev,
		"profileChanges":             profileChanges,
		"profileCommandRevision":     profilecommandrev2,
		"serverTime":                 time.Now().Format(time.RFC3339),
		"responseVersion":            1,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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

func LoadOffers(filePath string) map[string]Offer {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Failed to open file %s: %v", filePath, err)
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalf("Failed to read file %s: %v", filePath, err)
	}

	var offers map[string]Offer
	if err := json.Unmarshal(bytes, &offers); err != nil {
		log.Fatalf("Failed to unmarshal JSON offers from file %s: %v", filePath, err)
	}

	return offers
}
