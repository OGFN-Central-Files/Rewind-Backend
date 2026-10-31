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

type Operations struct{}

func (f *Operations) HandleQueryProfile(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimPrefix(r.URL.Path, "/fortnite/api/game/v2/")
	parts := strings.Split(rawPath, "/")

	if len(parts) < 4 {
		http.Error(w, "Invalid URL structure. Expected format: /profile/{accountId}/client/{operation}", http.StatusBadRequest)
		return
	}

	accountId := parts[1]
	profileId := r.URL.Query().Get("profileId")

	funcs := &services.Functions{}
	profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")

	var existingProfile models.Profile
	err := profilesCollection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&existingProfile)
	if err != nil {
		funcs.CreateError(
			"errors.com.epicgames.common.not_found",
			"Sorry the resource you were trying to find could not be found",
			nil, 1004, "any", "prod",
			"Sorry the resource you were trying to find could not be found",
			"not_found", w,
		)
		return
	}

	profileDataRaw, exists := existingProfile.ProfileData[profileId]
	if !exists {
		funcs.CreateError(
			"errors.com.epicgames.modules.profiles.profile_not_found",
			"Profile not found",
			[]string{profileId}, 12813,
			"any", "prod", "Profile not found",
			"not_found", w,
		)
		return
	}

	profileData, _ := profileDataRaw.(map[string]interface{})
	profileRevision := intOrDefault(profileData["rvn"], 1)
	commandRevision := intOrDefault(profileData["commandRevision"], 1)

	var multiUpdate []map[string]interface{}

	if profileId == "common_core" {
		if giftUserRaw, ok := global.GiftReceivedUsers[accountId]; ok {
			giftUser, ok := giftUserRaw.(map[string]interface{})
			if ok && giftUser["bReceivedGift"] == true {
				giftUser["bReceivedGift"] = false

				if athenaRaw, ok := existingProfile.ProfileData["athena"]; ok {
					if athenaProfile, ok := athenaRaw.(map[string]interface{}); ok {
						multiUpdate = []map[string]interface{}{
							{
								"profileRevision":            profileRevision,
								"profileId":                  "athena",
								"profileChangesBaseRevision": profileRevision,
								"profileChanges": []map[string]interface{}{
									{
										"changeType": "fullProfileUpdate",
										"profile":    athenaProfile,
									},
								},
								"profileCommandRevision": profileRevision,
							},
						}
					}
				}
			}
		}
	}

	if profileId == "athena" {
		if giftUserRaw, ok := global.GiftReceivedUsers[accountId]; ok {
			giftUser, ok := giftUserRaw.(map[string]interface{})
			if ok && giftUser["bReceivedGift"] == true {
				giftUser["bReceivedGift"] = false

				if athenaRaw, ok := existingProfile.ProfileData["common_core"]; ok {
					if athenaProfile, ok := athenaRaw.(map[string]interface{}); ok {
						multiUpdate = []map[string]interface{}{
							{
								"profileRevision":            profileRevision,
								"profileId":                  "common_core",
								"profileChangesBaseRevision": profileRevision,
								"profileChanges": []map[string]interface{}{
									{
										"changeType": "fullProfileUpdate",
										"profile":    athenaProfile,
									},
								},
								"profileCommandRevision": profileRevision,
							},
						}
					}
				}
			}
		}
	}

	response := map[string]interface{}{
		"profileRevision":            profileRevision,
		"profileId":                  profileId,
		"profileChangesBaseRevision": profileRevision,
		"profileChanges": []map[string]interface{}{
			{
				"changeType": "fullProfileUpdate",
				"profile":    profileData,
			},
		},
		"profileCommandRevision": commandRevision,
		"serverTime":             time.Now().UTC().Format(time.RFC3339),
		"multiUpdate":            multiUpdate,
		"responseVersion":        1,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func intOrDefault(val interface{}, def int) int {
	if f, ok := val.(float64); ok {
		return int(f)
	}
	return def
}
