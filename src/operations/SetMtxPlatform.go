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

func (f *Operations) SetMtxPlatform(w http.ResponseWriter, r *http.Request) {
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

	profilesCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var existingProfile *models.Profile
	err_profile := profilesCollection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&existingProfile)
	if err_profile != nil {
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

	profileData, _ := existingProfile.ProfileData[profileId].(map[string]interface{})

	MultiUpdate := []interface{}{}
	ApplyProfileChanges := []interface{}{}
	BaseRevision := profileData["rvn"]

	response := map[string]interface{}{
		"profileRevision":            profileData["rvn"],
		"profileId":                  profileId,
		"profileChangesBaseRevision": BaseRevision,
		"profileChanges":             ApplyProfileChanges,
		"profileCommandRevision":     profileData["commandRevision"],
		"serverTime":                 time.Now().UTC().Format(time.RFC3339),
		"multiUpdate":                MultiUpdate,
		"responseVersion":            1,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
