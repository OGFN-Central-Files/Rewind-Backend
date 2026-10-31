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

func (f *Operations) RemoveGiftBox(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Path
	rawPath = strings.TrimPrefix(rawPath, "/fortnite/api/game/v2/")
	parts := strings.Split(rawPath, "/")

	if len(parts) < 4 {
		http.Error(w, "Invalid URL structure. Expected format: /profile/{accountId}/client/{operation}", http.StatusBadRequest)
		return
	}

	accountId := parts[1]
	profileId := r.URL.Query().Get("profileId")
	queryRevision := r.URL.Query().Get("rvn")
	funcs := &services.Functions{}

	profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")
	var profile *models.Profile
	err := profilesCollection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&profile)
	if err != nil {
		funcs.CreateError(
			"errors.com.epicgames.common.not_found",
			"Unable to find profile",
			[]string{},
			1004, "any", "prod",
			"Unable to find profile",
			"not_found", w)
		return
	}

	if profileId != "common_core" && profileId != "profile0" {
		funcs.CreateError(
			"errors.com.epicgames.modules.profiles.invalid_command",
			"RemoveGiftBox is not valid on this profile",
			[]string{"RemoveGiftBox", profileId},
			12801, "any", "prod",
			"RemoveGiftBox is not valid on this profile",
			"invalid_command", w)
		return
	}

	profileData, ok := profile.ProfileData[profileId].(map[string]interface{})
	if !ok {
		funcs.CreateError(
			"errors.com.epicgames.modules.profiles.operation_forbidden",
			"Unable to find template configuration for profile",
			[]string{profileId}, 12813, "any", "prod",
			"Unable to find template configuration for profile",
			"operation_forbidden", w)
		return
	}

	var body struct {
		GiftBoxItemId  string   `json:"giftBoxItemId"`
		GiftBoxItemIds []string `json:"giftBoxItemIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		funcs.CreateError(
			"errors.com.epicgames.validation.validation_failed",
			"Invalid request body",
			[]string{}, 1040, "any", "prod",
			"Invalid request body",
			"validation_failed", w)
		return
	}

	items := profileData["items"].(map[string]interface{})
	var changes []map[string]interface{}
	baseRevision, _ := funcs.GetRVN(profileData, "rvn")
	profileRevisionCheck, _ := funcs.GetRVN(profileData, "rvn")
	//commandRevision := profileData["commandRevision"].(int)

	if body.GiftBoxItemId != "" {
		item, exists := items[body.GiftBoxItemId]
		if !exists || !strings.HasPrefix(item.(map[string]interface{})["templateId"].(string), "GiftBox:") {
			funcs.CreateError(
				"errors.com.epicgames.fortnite.id_invalid",
				"The specified item id is not a giftbox.",
				[]string{body.GiftBoxItemId}, 16027, "any", "prod",
				"The specified item id is not a giftbox.",
				"id_invalid", w)
			return
		}
		delete(items, body.GiftBoxItemId)
		changes = append(changes, map[string]interface{}{
			"changeType": "itemRemoved",
			"itemId":     body.GiftBoxItemId,
		})
	}

	for _, itemId := range body.GiftBoxItemIds {
		item, exists := items[itemId]
		if !exists {
			continue
		}
		if !strings.HasPrefix(item.(map[string]interface{})["templateId"].(string), "GiftBox:") {
			continue
		}
		delete(items, itemId)
		changes = append(changes, map[string]interface{}{
			"changeType": "itemRemoved",
			"itemId":     itemId,
		})
	}

	if len(changes) > 0 {
		nextBaseRevision, _ := funcs.GetAndMultiplyRVN(profileData, "rvn", 1)
		nextCmdRevision, _ := funcs.GetAndMultiplyRVN(profileData, "commandRevision", 1)
		profileData["rvn"] = nextBaseRevision
		profileData["commandRevision"] = nextCmdRevision
		profileData["updated"] = time.Now().UTC().Format(time.RFC3339)

		_, err := profilesCollection.UpdateOne(context.Background(), bson.M{"accountid": accountId}, bson.M{
			"$set": bson.M{"profiledata": profile.ProfileData},
		})
		if err != nil {
			funcs.CreateError(
				"errors.com.epicgames.modules.profiles.operation_forbidden",
				"Failed to save profile",
				[]string{}, 12813, "any", "prod",
				"Failed to save profile",
				"operation_forbidden", w)
			return
		}
	}

	if queryRevision != "" && queryRevision != profileRevisionCheck {
		changes = []map[string]interface{}{
			{
				"changeType": "fullProfileUpdate",
				"profile":    profileData,
			},
		}
	}

	rvn, _ := funcs.GetRVN(profileData, "rvn")
	response := map[string]interface{}{
		"profileRevision":            rvn,
		"profileId":                  profileId,
		"profileChangesBaseRevision": baseRevision,
		"profileChanges":             changes,
		"profileCommandRevision":     profileData["commandRevision"],
		"serverTime":                 time.Now().UTC().Format(time.RFC3339),
		"responseVersion":            1,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
