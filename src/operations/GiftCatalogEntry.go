package operations

import (
	"context"
	"encoding/json"
	"go-backend/src/database/models"
	XMPP "go-backend/src/definitions/xmpp"
	"go-backend/src/global"
	"go-backend/src/services"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func (f *Operations) HandleGiftCatalogEntry(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimPrefix(r.URL.Path, "/fortnite/api/game/v2/")
	parts := strings.Split(rawPath, "/")
	if len(parts) < 4 {
		http.Error(w, "Invalid URL structure", http.StatusBadRequest)
		return
	}

	accountId := parts[1]
	funcs := &services.Functions{}
	accounts := global.MongoDBClient.Database("fortnite").Collection("accounts")
	profiles := global.MongoDBClient.Database("fortnite").Collection("profiles")

	var sender models.Account
	if err := accounts.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&sender); err != nil {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	var senderProfile models.Profile
	if err := profiles.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&senderProfile); err != nil {
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		funcs.Log(services.WARNING, "LogMCP", "RequestBody Invalid or not found")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	offerId, _ := body["offerId"].(string)
	giftWrap, _ := body["giftWrapTemplateId"].(string)
	personalMsg, _ := body["personalMessage"].(string)
	if giftWrap == "" {
		giftWrap = "GiftBox:gb_default"
	}

	receiversRaw, ok := body["receiverAccountIds"]
	if !ok {
		http.Error(w, "'receiverAccountIds' is required", http.StatusBadRequest)
		return
	}

	receivers, ok := receiversRaw.([]interface{})
	if !ok {
		http.Error(w, "'receiverAccountIds' must be an array", http.StatusBadRequest)
		return
	}

	if len(receivers) != 1 {
		http.Error(w, "Only one receiver is allowed", http.StatusBadRequest)
		return
	}

	receiverId, ok := receivers[0].(string)
	if !ok {
		http.Error(w, "Receiver ID must be a string", http.StatusBadRequest)
		return
	}

	var receiverProfile models.Profile
	if err := profiles.FindOne(context.Background(), bson.M{"accountid": receiverId}).Decode(&receiverProfile); err != nil {
		funcs.Log(services.WARNING, "LogMCP", "Receiver not found")
		http.Error(w, "Receiver not found", http.StatusNotFound)
		return
	}

	offer, err := funcs.FindOfferId(global.Catalog, offerId)
	if err != nil {
		funcs.Log(services.WARNING, "LogMCP", "Offer not found")
		http.Error(w, "Offer not found", http.StatusBadRequest)
		return
	}

	price := offer.Prices[0].FinalPrice
	commonCore := senderProfile.ProfileData["common_core"].(map[string]interface{})
	items := commonCore["items"].(map[string]interface{})

	for _, item := range commonCore["items"].(map[string]interface{}) {
		if m, ok := item.(map[string]interface{}); ok {
			if templateId, ok := m["templateId"].(string); ok && templateId == "Currency:MtxPurchased" {
				rawQty := item.(map[string]interface{})["quantity"]
				quantity, err := funcs.ToFloat64(rawQty)
				if err != nil {
					funcs.CreateError(
						"errors.com.epicgames.mcp.invalid_quantity_type",
						"Invalid quantity type for item.",
						[]string{},
						-1,
						"any",
						"prod",
						"Invalid quantity type for item.",
						"invalid_quantity_type",
						w,
					)
					return
				}

				if quantity < price {
					funcs.CreateError(
						"errors.com.epicgames.mcp.not_enough_mtx",
						"You do not have enough MTX to buy this item.",
						[]string{},
						-1,
						"any",
						"prod",
						"You do not have enough MTX to buy this item.",
						"not_enough_mtx",
						w,
					)
					return
				}

				item.(map[string]interface{})["quantity"] = quantity - price

				break
			}
		}
	}

	commonCore["items"] = items
	senderProfile.ProfileData["common_core"] = commonCore

	rvn, _ := funcs.GetAndMultiplyRVN(commonCore, "rvn", 1)
	cmdRvn, _ := funcs.GetAndMultiplyRVN(commonCore, "commandRevision", 1)

	profiles.UpdateOne(context.Background(), bson.M{"accountid": accountId}, bson.M{"$set": bson.M{"profiledata.common_core": commonCore}})

	athena := receiverProfile.ProfileData["athena"].(map[string]interface{})
	commonCore = receiverProfile.ProfileData["common_core"].(map[string]interface{})

	items = athena["items"].(map[string]interface{})
	common_items := commonCore["items"].(map[string]interface{})

	giftId := funcs.MakeID()
	giftBox := map[string]interface{}{
		"templateId": giftWrap,
		"attributes": map[string]interface{}{
			"fromAccountId": accountId,
			"lootList":      []interface{}{},
			"params": map[string]interface{}{
				"userMessage": personalMsg,
			},
			"level":    1,
			"giftedOn": time.Now().Format(time.RFC3339),
		},
		"quantity": 1,
	}

	lootList := giftBox["attributes"].(map[string]interface{})["lootList"].([]interface{})
	now := time.Now().Format(time.RFC3339)
	for _, grant := range offer.ItemGrants {
		itemId := grant.TemplateID
		items[itemId] = map[string]interface{}{
			"templateId":  itemId,
			"attributes":  map[string]interface{}{"item_seen": false},
			"quantity":    1,
			"purchasedAt": now,
		}

		lootList = append(lootList, map[string]interface{}{
			"itemType":    itemId,
			"itemGuid":    itemId,
			"itemProfile": "athena",
			"quantity":    1,
		})
	}

	giftBox["attributes"].(map[string]interface{})["lootList"] = lootList
	common_items[giftId] = giftBox

	athena["items"] = items
	commonCore["items"] = common_items

	funcs.GetAndMultiplyRVN(athena, "rvn", 1)
	funcs.GetAndMultiplyRVN(athena, "commandRevision", 1)

	funcs.GetAndMultiplyRVN(commonCore, "rvn", 1)
	funcs.GetAndMultiplyRVN(commonCore, "commandRevision", 1)

	receiverProfile.ProfileData["athena"] = athena
	athena["updatedAt"] = time.Now().UTC()
	profiles.UpdateOne(context.Background(), bson.M{"accountid": receiverId}, bson.M{"$set": bson.M{"profiledata.athena": athena, "profiledata.common_core": commonCore}})

	funcs.Log(services.WARNING, "LogMCP", "sent!")
	response := map[string]interface{}{
		"profileRevision":            rvn,
		"profileId":                  "common_core",
		"profileChangesBaseRevision": rvn,
		"profileChanges": []map[string]interface{}{
			{
				"changeType": "itemAdded",
				"itemId":     giftId,
				"item":       giftBox,
			},
		},
		"notifications":          []interface{}{},
		"profileCommandRevision": cmdRvn,
		"serverTime":             now,
		"multiUpdate":            []interface{}{},
	}

	global.GiftReceivedUsers[receiverProfile.AccountID] = map[string]interface{}{
		"bReceivedGift": true,
	}

	xmpp := XMPP.XmppService{}
	xmpp.SendXmppMessageToClient(receiverProfile.AccountID, map[string]interface{}{
		"type":      "com.epicgames.gift.received",
		"payload":   map[string]interface{}{},
		"timestamp": time.Now().Unix(),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
