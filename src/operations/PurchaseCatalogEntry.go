package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"go-backend/src/database/models"
	"go-backend/src/global"
	"go-backend/src/services"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func (f *Operations) HandlePurchaseCatalogEntry(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Path
	rawPath = strings.TrimPrefix(rawPath, "/fortnite/api/game/v2/")
	parts := strings.Split(rawPath, "/")

	if len(parts) < 4 {
		http.Error(w, "Invalid URL structure. Expected format: /profile/{accountId}/client/{operation}", http.StatusBadRequest)
		return
	}

	accountId := parts[1]
	profileId := r.URL.Query().Get("profileId")
	funcs := &services.Functions{}

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var existingAccount *models.Account
	err := accountsCollection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&existingAccount)
	if err != nil {
		funcs.Log(services.INFO, "Account ID", accountId)
		funcs.CreateError(
			"errors.com.epicgames.common.not_found",
			"Sorry the resource you were trying to find could not be found",
			[]string{},
			1004,
			"any",
			"prod",
			"Sorry the resource you were trying to find could not be found",
			"not_found",
			w,
		)
		return
	}

	profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")
	var existingProfile *models.Profile
	err = profilesCollection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&existingProfile)
	if err != nil {
		funcs.Log(services.INFO, "Account ID", accountId)
		funcs.CreateError(
			"errors.com.epicgames.common.not_found",
			"Sorry the resource you were trying to find could not be found",
			[]string{},
			1004,
			"any",
			"prod",
			"Sorry the resource you were trying to find could not be found",
			"not_found",
			w,
		)
		return
	}

	profiledata, ok := existingProfile.ProfileData[profileId].(map[string]interface{})
	if !ok {
		funcs.CreateError(
			"errors.com.epicgames.modules.profiles.profile_not_found",
			"Profile not found",
			[]string{profileId},
			12813,
			"any",
			"prod",
			"Profile not found",
			"not_found",
			w,
		)
		return
	}

	var requestBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	offerId, ok := requestBody["offerId"].(string)
	if !ok {
		funcs.CreateError(
			"errors.com.epicgames.validation.validation_failed",
			"Validation Failed. 'offerId' is not a string.",
			[]string{"offerId"},
			1040,
			"any",
			"prod",
			"Validation Failed. 'offerId' is not a string.",
			"validation_failed",
			w,
		)
		return
	}

	purchaseQuantity, ok := requestBody["purchaseQuantity"].(float64)
	if !ok {
		funcs.CreateError(
			"errors.com.epicgames.validation.validation_failed",
			"Validation Failed. 'purchaseQuantity' is not a number.",
			[]string{"purchaseQuantity"},
			1040,
			"any",
			"prod",
			"Validation Failed. 'purchaseQuantity' is not a number.",
			"validation_failed",
			w,
		)
		return
	}

	if purchaseQuantity < 1 {
		funcs.CreateError(
			"errors.com.epicgames.validation.validation_failed",
			"Validation Failed. 'purchaseQuantity' is not greater than 0.",
			[]string{"purchaseQuantity"},
			1040,
			"any",
			"prod",
			"Validation Failed. 'purchaseQuantity' is not greater than 0.",
			"validation_failed",
			w,
		)
		return
	}

	offer, err := funcs.FindOfferId(global.Catalog, offerId)
	if err != nil {
		funcs.CreateError(
			"errors.com.epicgames.fortnite.id_invalid",
			fmt.Sprintf("Offer ID '%s' not found", offerId),
			[]string{offerId},
			16027,
			"any",
			"prod",
			fmt.Sprintf("Offer ID '%s' not found", offerId),
			"not_found",
			w,
		)
		return
	}

	profile := profiledata
	athena, ok := existingProfile.ProfileData["athena"].(map[string]interface{})
	common_core, ok_common := existingProfile.ProfileData["common_core"].(map[string]interface{})
	if !ok || !ok_common {
		athena = map[string]interface{}{
			"items":           map[string]interface{}{},
			"stats":           map[string]interface{}{"attributes": map[string]interface{}{}},
			"rvn":             0,
			"commandRevision": 0,
		}
		existingProfile.ProfileData["athena"] = athena
	}

	if profile["stats"] == nil {
		profile["stats"] = map[string]interface{}{}
	}
	if profile["stats"].(map[string]interface{})["attributes"] == nil {
		profile["stats"].(map[string]interface{})["attributes"] = map[string]interface{}{}
	}
	if profile["stats"].(map[string]interface{})["attributes"].(map[string]interface{})["current_mtx_platform"] == nil {
		profile["stats"].(map[string]interface{})["attributes"].(map[string]interface{})["current_mtx_platform"] = "Epic"
	}

	profileRvn, _ := funcs.GetRVN(athena, "rvn")
	commandRvn, _ := funcs.GetRVN(athena, "commandRevision")
	multiUpdate := []map[string]interface{}{
		{
			"profileRevision":            profileRvn,
			"profileId":                  "athena",
			"profileChangesBaseRevision": profileRvn,
			"profileChanges":             []map[string]interface{}{},
			"profileCommandRevision":     commandRvn,
		},
	}

	notifications := []map[string]interface{}{}
	applyProfileChanges := []map[string]interface{}{}

	if offer.Prices[0].CurrencyType == "MtxCurrency" {
		price := offer.Prices[0].FinalPrice
		paid := false

		for _, item := range common_core["items"].(map[string]interface{}) {
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
					paid = true

					break
				}
			}
		}

		if !paid {
			funcs.CreateError(
				"errors.com.epicgames.mcp.no_valid_items",
				"No valid items found for MTX purchase.",
				[]string{},
				-1,
				"any",
				"prod",
				"No valid items found for MTX purchase.",
				"no_valid_items",
				w,
			)
		}
	}

	for _, itemGrant := range offer.ItemGrants {
		itemID := itemGrant.TemplateID

		itemExists := false
		for _, item := range athena["items"].(map[string]interface{}) {
			if strings.ToLower(item.(map[string]interface{})["templateId"].(string)) == strings.ToLower(itemID) {
				itemExists = true
				break
			}
		}

		if !itemExists {
			notifications = append(notifications, map[string]interface{}{
				"type":    "CatalogPurchase",
				"primary": true,
				"lootResult": map[string]interface{}{
					"items": []interface{}{},
				},
			})

			lootResult := notifications[0]["lootResult"].(map[string]interface{})
			lootResultItems := lootResult["items"].([]interface{})

			lootResultItems = append(lootResultItems, map[string]interface{}{
				"itemType":    itemID,
				"itemGuid":    itemID,
				"itemProfile": "athena",
				"quantity":    1,
			})

			lootResult["items"] = lootResultItems

			athena["items"].(map[string]interface{})[itemID] = map[string]interface{}{
				"templateId":  itemID,
				"purchasedAt": time.Now().Format(time.RFC3339),
				"attributes": map[string]interface{}{
					"item_seen": false,
					"variants":  []interface{}{},
				},
				"quantity": 1,
			}

			multiUpdate[0]["profileChanges"] = append(multiUpdate[0]["profileChanges"].([]map[string]interface{}), map[string]interface{}{
				"changeType": "itemAdded",
				"itemId":     itemID,
				"item":       athena["items"].(map[string]interface{})[itemID],
			})

			applyProfileChanges = append(applyProfileChanges, map[string]interface{}{
				"changeType": "itemQuantityChanged",
				"itemId":     itemID,
				"quantity":   athena["items"].(map[string]interface{})[itemID],
			})
		}
	}

	profilerev2, _ := funcs.GetAndMultiplyRVN(profile, "rvn", 2)
	profilecommandrev2, _ := funcs.GetAndMultiplyRVN(profile, "commandRevision", 2)

	profile["rvn"] = profilerev2
	profile["commandRevision"] = profilecommandrev2
	profile["updatedAt"] = time.Now().Format(time.RFC3339)

	athenarev2, _ := funcs.GetAndMultiplyRVN(athena, "rvn", 2)
	athenacommandrev2, _ := funcs.GetAndMultiplyRVN(athena, "commandRevision", 2)
	athena["rvn"] = athenarev2
	athena["commandRevision"] = athenacommandrev2
	athena["updatedAt"] = time.Now().Format(time.RFC3339)

	corerev2, _ := funcs.GetAndMultiplyRVN(common_core, "rvn", 2)
	corecommandrev2, _ := funcs.GetAndMultiplyRVN(common_core, "commandRevision", 2)
	common_core["rvn"] = corerev2
	common_core["commandRevision"] = corecommandrev2
	common_core["updatedAt"] = time.Now().Format(time.RFC3339)

	applyProfileChanges = append(applyProfileChanges, map[string]interface{}{
		"changeType": "fullProfileUpdate",
		"profile":    profile,
	})

	updateResult, err := profilesCollection.UpdateOne(
		context.Background(),
		bson.M{"accountid": accountId},
		bson.M{"$set": bson.M{
			fmt.Sprintf("profiledata.%s", profileId): profile,
			"profiledata.athena":                     athena,
			"profiledata.common_core":                common_core,
		}},
	)
	if err != nil {
		funcs.Log(services.ERROR, "Failed to update profile:", err.Error())
		http.Error(w, "Failed to update profile", http.StatusInternalServerError)
		return
	}

	if updateResult.MatchedCount == 0 {
		funcs.Log(services.ERROR, "No document matched the filter for account:", accountId)
		http.Error(w, "No document found to update", http.StatusNotFound)
		return
	}

	athenarev3, _ := funcs.GetAndMultiplyRVN(athena, "rvn", 2)
	athenacommandrev3, _ := funcs.GetAndMultiplyRVN(athena, "commandRevision", 2)
	athena["rvn"] = athenarev3
	athena["commandRevision"] = athenacommandrev3
	athena["updatedAt"] = time.Now().Format(time.RFC3339)

	multiUpdate[0]["profileRevision"] = athena["rvn"]
	multiUpdate[0]["profileCommandRevision"] = athena["commandRevision"]

	_, err = profilesCollection.UpdateOne(
		context.Background(),
		bson.M{"accountid": accountId},
		bson.M{"$set": bson.M{
			"profiledata.athena": athena,
		}},
	)
	if err != nil {
		funcs.Log(services.ERROR, "Failed to update profile:", err.Error())
		http.Error(w, "Failed to update profile", http.StatusInternalServerError)
		return
	}

	baseRev, _ := funcs.GetRVN(profile, "rvn")
	baseCmdRev, _ := funcs.GetRVN(profile, "commandRevision")
	changesBaseRev, _ := funcs.GetRVN(profile, "rvn")
	response := map[string]interface{}{
		"profileRevision":            baseRev,
		"profileId":                  profileId,
		"profileChangesBaseRevision": changesBaseRev,
		"profileChanges":             applyProfileChanges,
		"notifications":              notifications,
		"profileCommandRevision":     baseCmdRev,
		"serverTime":                 time.Now().Format(time.RFC3339),
		"multiUpdate":                multiUpdate,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
