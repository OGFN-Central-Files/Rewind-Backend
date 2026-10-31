package operations

import (
	"context"
	"encoding/json"
	"go-backend/src/database/models"
	"go-backend/src/global"
	"go-backend/src/services"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func (f *Operations) MarkNewQuestNotificationSent(w http.ResponseWriter, r *http.Request) {
	funcs := &services.Functions{}
	rawPath := r.URL.Path
	rawPath = strings.TrimPrefix(rawPath, "/fortnite/api/game/v2/")
	parts := strings.Split(rawPath, "/")

	if len(parts) < 4 {
		http.Error(w, "Invalid URL structure. Expected format: /profile/{accountId}/client/{operation}", http.StatusBadRequest)
		return
	}

	accountId := parts[1]
	profileId := r.URL.Query().Get("profileId")

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

	profileData, ok := existingProfile.ProfileData[profileId].(map[string]interface{})
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

	var statChanged bool
	if r.Body != nil {
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if itemIds, ok := body["itemIds"].([]interface{}); ok {
			for _, itemId := range itemIds {
				id := itemId.(string)

				if items, ok := profileData["items"].(map[string]interface{}); ok {
					if item, exists := items[id].(map[string]interface{}); exists {
						item["attributes"].(map[string]interface{})["sent_new_notification"] = true
					}
				}

				if changes, ok := profileData["changes"].([]interface{}); ok {
					profileData["changes"] = append(changes, map[string]interface{}{
						"changeType":     "itemAttrChanged",
						"itemId":         id,
						"attributeName":  "sent_new_notification",
						"attributeValue": true,
					})
				} else {
					profileData["changes"] = []interface{}{
						map[string]interface{}{
							"changeType":     "itemAttrChanged",
							"itemId":         id,
							"attributeName":  "sent_new_notification",
							"attributeValue": true,
						},
					}
				}
			}
			statChanged = true
		}
	}

	if statChanged {
		rvnOne, _ := funcs.GetAndMultiplyRVN(profileData, "rvn", 1)
		profileData["rvn"] = rvnOne

		cmdRevOne, _ := funcs.GetAndMultiplyRVN(profileData, "commandRevision", 1)
		profileData["commandRevision"] = cmdRevOne

		_, err := profilesCollection.UpdateOne(
			context.Background(),
			bson.M{"accountid": accountId},
			bson.M{"$set": bson.M{"profiledata": existingProfile.ProfileData}},
		)
		if err != nil {
			http.Error(w, "Failed to update profile", http.StatusInternalServerError)
			return
		}
	}

	queryRevision := -1
	if rvn := r.URL.Query().Get("rvn"); rvn != "" {
		if rev, err := strconv.Atoi(rvn); err == nil {
			queryRevision = rev
		}
	}

	baseRevision, _ := funcs.GetRVN(profileData, "RVN")
	var profileChanges []map[string]interface{}
	if int(queryRevision) != baseRevision {
		profileChanges = append(profileChanges, map[string]interface{}{
			"changeType": "fullProfileUpdate",
			"profile":    profileData,
		})
	} else {
		profileChanges = append(profileChanges, map[string]interface{}{
			"changeType": "fullProfileUpdate",
			"profile":    profileData,
		})
	}

	commandRevisionResponse, _ := funcs.GetRVN(profileData, "commandRevision")
	profileRevisionResponse, _ := funcs.GetRVN(profileData, "rvn")
	response := map[string]interface{}{
		"profileRevision":            profileRevisionResponse,
		"profileId":                  profileId,
		"profileChangesBaseRevision": baseRevision,
		"profileChanges":             profileChanges,
		"profileCommandRevision":     commandRevisionResponse,
		"serverTime":                 time.Now().UTC().Format(time.RFC3339),
		"responseVersion":            1,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
