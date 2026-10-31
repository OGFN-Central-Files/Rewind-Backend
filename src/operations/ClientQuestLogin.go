package operations

import (
	"context"
	"encoding/json"
	"go-backend/src/database/models"
	"go-backend/src/global"
	"go-backend/src/services"
	"net/http"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (f *Operations) ClientQuestLogin(w http.ResponseWriter, r *http.Request) {
	funcs := &services.Functions{}
	rawPath := r.URL.Path
	rawPath = strings.TrimPrefix(rawPath, "/fortnite/api/game/v2/")
	parts := strings.Split(rawPath, "/")

	if len(parts) < 4 {
		funcs.Log(services.ERROR, "Invalid URL structure", rawPath)
		http.Error(w, "Invalid URL structure. Expected format: /profile/{accountId}/client/{operation}", http.StatusBadRequest)
		return
	}

	accountId := parts[1]
	profileId := r.URL.Query().Get("profileId")

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var existingAccount models.Account
	err := accountsCollection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&existingAccount)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			funcs.Log(services.ERROR, "Account not found", accountId)
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
		funcs.Log(services.ERROR, "Error finding account", err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")
	var existingProfile models.Profile
	err = profilesCollection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&existingProfile)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			funcs.Log(services.ERROR, "Profile not found", accountId)
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
		funcs.Log(services.ERROR, "Error finding profile", err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	profileData, ok := existingProfile.ProfileData[profileId].(map[string]interface{})
	if !ok {
		funcs.Log(services.ERROR, "Profile data is invalid for profileId", profileId)
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

	questData, err := loadQuestData("./static/battlepass/quests.json")
	if err != nil {
		funcs.Log(services.ERROR, "Failed to load quest data", err.Error())
		http.Error(w, "Failed to load quest data", http.StatusInternalServerError)
		return
	}

	applyProfileChanges := []map[string]interface{}{}
	notifications := []map[string]interface{}{}
	baseRevision, _ := funcs.GetRVN(profileData, "rvn")
	statChanged := true

	if profileId == "athena" {

		dailyQuestIDs := questData["Daily"].([]interface{})

		for key, item := range profileData["items"].(map[string]interface{}) {
			itemMap := item.(map[string]interface{})
			if strings.HasPrefix(itemMap["templateId"].(string), "Quest:AthenaDaily") && itemMap["attributes"].(map[string]interface{})["quest_state"] == "Claimed" {
				delete(profileData["items"].(map[string]interface{}), key)
				applyProfileChanges = append(applyProfileChanges, map[string]interface{}{
					"changeType": "itemRemoved",
					"itemId":     key,
				})
			}
		}

		activeQuestCount := 0
		for _, item := range profileData["items"].(map[string]interface{}) {
			itemMap := item.(map[string]interface{})
			if strings.HasPrefix(itemMap["templateId"].(string), "Quest:AthenaDaily") && itemMap["attributes"].(map[string]interface{})["quest_state"] != "Claimed" {
				activeQuestCount++
			}
		}

		if activeQuestCount < 3 {
			remainingSlots := 3 - activeQuestCount
			selectedQuests := []interface{}{}

			for len(selectedQuests) < remainingSlots && len(dailyQuestIDs) > 0 {
				quest := dailyQuestIDs[0]
				dailyQuestIDs = dailyQuestIDs[1:]

				isActive := false
				for _, item := range profileData["items"].(map[string]interface{}) {
					itemMap := item.(map[string]interface{})
					if itemMap["templateId"] == quest.(map[string]interface{})["templateId"] && itemMap["attributes"].(map[string]interface{})["quest_state"] != "Claimed" {
						isActive = true
						break
					}
				}

				if !isActive {
					selectedQuests = append(selectedQuests, quest)
				}
			}

			for _, quest := range selectedQuests {
				questMap := quest.(map[string]interface{})
				newQuestID := questMap["templateId"].(string)

				profileData["items"].(map[string]interface{})[newQuestID] = map[string]interface{}{
					"templateId": questMap["templateId"],
					"attributes": map[string]interface{}{
						"creation_time":          time.Now().Format(time.RFC3339),
						"level":                  -1,
						"item_seen":              false,
						"sent_new_notification":  false,
						"xp_reward_scalar":       1,
						"quest_state":            "Active",
						"last_state_change_time": time.Now().Format(time.RFC3339),
						"max_level_bonus":        0,
						"xp":                     0,
						"favorite":               false,
					},
					"quantity": 1,
				}

				applyProfileChanges = append(applyProfileChanges, map[string]interface{}{
					"changeType": "itemAdded",
					"itemId":     newQuestID,
					"item":       profileData["items"].(map[string]interface{})[newQuestID],
				})
			}
		}
	}

	if statChanged {
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
	}

	profilerev2, _ := funcs.GetAndMultiplyRVN(profileData, "rvn", 1)
	profilecommandrev2, _ := funcs.GetAndMultiplyRVN(profileData, "commandRevision", 1)

	applyProfileChanges = []map[string]interface{}{
		{
			"changeType": "fullProfileUpdate",
			"profile":    profileData,
		},
	}

	profileData["rvn"] = profilerev2
	profileData["commandRevision"] = profilecommandrev2
	profileData["updated"] = time.Now().Format(time.RFC3339)

	response := map[string]interface{}{
		"profileRevision":            profilerev2,
		"profileId":                  profileId,
		"profileChangesBaseRevision": baseRevision,
		"profileChanges":             applyProfileChanges,
		"notifications":              notifications,
		"profileCommandRevision":     profilecommandrev2,
		"serverTime":                 time.Now().Format(time.RFC3339),
		"responseVersion":            1,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func loadQuestData(filename string) (map[string]interface{}, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return nil, err
	}

	return data, nil
}
