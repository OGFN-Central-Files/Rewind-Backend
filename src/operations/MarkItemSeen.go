package operations

import (
	"context"
	"encoding/json"
	"go-backend/src/database/models"
	"go-backend/src/global"
	"go-backend/src/services"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func (f *Operations) MarkItemSeen(w http.ResponseWriter, r *http.Request) {
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

	profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")
	var existingProfile *models.Profile
	err := profilesCollection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&existingProfile)
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

	if profileId != "athena" {
		funcs.CreateError(
			"errors.com.epicgames.modules.profiles.invalid_command",
			"MarkItemSeen is not valid on this profile",
			[]string{"MarkItemSeen", profileId},
			12801,
			"any",
			"prod",
			"MarkItemSeen is not valid on this profile",
			"invalid_command",
			w,
		)
		return
	}

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

	var requestBody struct {
		ItemIds []string `json:"itemIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		funcs.CreateError(
			"errors.com.epicgames.validation.validation_failed",
			"Invalid request body",
			[]string{"itemIds"},
			1040,
			"any",
			"prod",
			"Invalid request body",
			"validation_failed",
			w,
		)
		return
	}

	if len(requestBody.ItemIds) == 0 {
		funcs.CreateError(
			"errors.com.epicgames.validation.validation_failed",
			"Validation Failed. 'itemIds' must be an array",
			[]string{"itemIds"},
			1040,
			"any",
			"prod",
			"Validation Failed. 'itemIds' must be an array",
			"validation_failed",
			w,
		)
		return
	}

	items := profileData["items"].(map[string]interface{})
	var applyProfileChanges []map[string]interface{}
	baseRevision, _ := funcs.GetRVN(profileData, "rvn")
	nextBaseRevision, _ := funcs.GetAndMultiplyRVN(profileData, "rvn", 1)

	CmdRevision, _ := funcs.GetRVN(profileData, "commandRevision")
	nextCmdRevision, _ := funcs.GetAndMultiplyRVN(profileData, "commandRevision", 1)

	for _, itemId := range requestBody.ItemIds {
		if item, exists := items[itemId]; exists {
			itemMap := item.(map[string]interface{})
			if itemMap["attributes"] == nil {
				itemMap["attributes"] = make(map[string]interface{})
			}
			itemMap["attributes"].(map[string]interface{})["item_seen"] = true

			applyProfileChanges = append(applyProfileChanges, map[string]interface{}{
				"changeType":     "itemAttrChanged",
				"itemId":         itemId,
				"attributeName":  "item_seen",
				"attributeValue": true,
			})
		}
	}

	if len(applyProfileChanges) > 0 {
		profileData["rvn"] = nextBaseRevision
		profileData["commandRevision"] = nextCmdRevision
		profileData["updated"] = time.Now().UTC().Format(time.RFC3339)

		_, err := profilesCollection.UpdateOne(
			context.Background(),
			bson.M{"accountid": accountId},
			bson.M{"$set": bson.M{"profiledata": existingProfile.ProfileData}},
		)
		if err != nil {
			funcs.Log(services.ERROR, "Failed to save profile:", err.Error())
			funcs.CreateError(
				"errors.com.epicgames.modules.profiles.operation_forbidden",
				"Failed to save profile",
				[]string{},
				12813,
				"any",
				"prod",
				"Failed to save profile",
				"operation_forbidden",
				w,
			)
			return
		}
	}

	response := map[string]interface{}{
		"profileRevision":            profileData["rvn"],
		"profileId":                  profileId,
		"profileChangesBaseRevision": baseRevision,
		"profileChanges":             applyProfileChanges,
		"profileCommandRevision":     CmdRevision,
		"serverTime":                 time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
