package operations

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"go-backend/src/database/models"
	"go-backend/src/global"
	"go-backend/src/services"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (f *Operations) EquipBattleRoyaleCustomization(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Path
	rawPath = strings.TrimPrefix(rawPath, "/fortnite/api/game/v2/")
	parts := strings.Split(rawPath, "/")
	funcs := &services.Functions{}
	if len(parts) < 4 {
		funcs.Log(services.ERROR, "Invalid URL structure", rawPath)
		funcs.CreateError(
			"errors.com.epicgames.validation.validation_failed",
			"Invalid URL structure. Expected format: /profile/{accountId}/client/{operation}",
			[]string{},
			1040,
			"any",
			"prod",
			"Invalid URL structure",
			"validation_failed",
			w,
		)
		return
	}

	accountId := parts[1]
	profileId := r.URL.Query().Get("profileId")
	rvn := r.URL.Query().Get("rvn")

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var existingAccount *models.Account
	err := accountsCollection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&existingAccount)
	if err != nil {
		funcs.Log(services.ERROR, "Failed to fetch account", accountId)
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
		funcs.Log(services.ERROR, "Failed to fetch profile", accountId)
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
		funcs.Log(services.ERROR, "Invalid profileId", profileId)
		funcs.CreateError(
			"errors.com.epicgames.validation.validation_failed",
			"EquipBRCustomization failed",
			[]string{},
			1040,
			"any",
			"prod",
			"EquipBRCustomization failed",
			"validation_failed",
			w,
		)
		return
	}

	profileData, ok := existingProfile.ProfileData[profileId].(map[string]interface{})
	if !ok {
		funcs.Log(services.ERROR, "Profile data not found", profileId)
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
		funcs.Log(services.ERROR, "Failed to parse request body", err.Error())
		funcs.CreateError(
			"errors.com.epicgames.validation.validation_failed",
			"Invalid request body",
			[]string{},
			1040,
			"any",
			"prod",
			"Invalid request body",
			"validation_failed",
			w,
		)
		return
	}

	if missingFields := checkFields([]string{"slotName"}, requestBody); len(missingFields) > 0 {
		funcs.Log(services.ERROR, "Missing required fields", "ww")
		funcs.CreateError(
			"errors.com.epicgames.validation.validation_failed",
			"EquipBRCustomization failed",
			missingFields,
			1040,
			"any",
			"prod",
			"EquipBRCustomization failed",
			"validation_failed",
			w,
		)
		return
	}

	itemToSlot, ok := requestBody["itemToSlot"].(string)
	if !ok {
		itemToSlot = ""
	}

	variantsUpdates, ok := requestBody["variantsUpdates"].([]interface{})
	if !ok {
		variantsUpdates = []interface{}{}
	}

	NextProfileRVN, _ := funcs.GetAndMultiplyRVN(profileData, "rvn", 1)
	NextProfileCommandRVN, _ := funcs.GetAndMultiplyRVN(profileData, "commandRevision", 1)

	profileData["rvn"] = NextProfileRVN
	profileData["commandRevision"] = NextProfileCommandRVN
	profileData["updated"] = time.Now().UTC().Format(time.RFC3339)

	applyProfileChanges := []map[string]interface{}{}

	if len(variantsUpdates) > 0 {
		items, ok := profileData["items"].(map[string]interface{})
		if ok {
			item, ok := items[itemToSlot].(map[string]interface{})
			if ok {
				attributes, ok := item["attributes"].(map[string]interface{})
				if ok {
					variants, ok := attributes["variants"].([]interface{})
					if ok {
						for _, variantUpdate := range variantsUpdates {
							variantUpdateMap, ok := variantUpdate.(map[string]interface{})
							if !ok {
								continue
							}

							channel, ok := variantUpdateMap["channel"].(string)
							if !ok {
								continue
							}

							active, ok := variantUpdateMap["active"].(string)
							if !ok {
								continue
							}

							for i, variant := range variants {
								variantMap, ok := variant.(map[string]interface{})
								if !ok {
									continue
								}

								variantChannel, ok := variantMap["channel"].(string)
								if !ok {
									continue
								}

								if variantChannel == channel {
									variantMap["active"] = active
									variants[i] = variantMap
									break
								}
							}
						}

						attributes["variants"] = variants
						item["attributes"] = attributes
						items[itemToSlot] = item
						profileData["items"] = items

						applyProfileChanges = append(applyProfileChanges, map[string]interface{}{
							"changeType":     "itemAttrChanged",
							"itemId":         itemToSlot,
							"attributeName":  "variants",
							"attributeValue": variants,
						})
					}
				}
			}
		}
	}

	slotName := requestBody["slotName"].(string)
	loadouts, ok := profileData["stats"].(map[string]interface{})["attributes"].(map[string]interface{})["loadouts"].(primitive.A)
	if !ok || len(loadouts) == 0 {
		funcs.Log(services.ERROR, "Invalid or empty loadouts", "wow")
		funcs.CreateError(
			"errors.com.epicgames.validation.validation_failed",
			"Invalid loadouts",
			[]string{},
			1040,
			"any",
			"prod",
			"Invalid loadouts",
			"validation_failed",
			w,
		)
		return
	}
	activeLoadoutId := loadouts[0].(string)
	templateId := itemToSlot

	switch slotName {
	case "Dance":
		indexWithinSlot, ok := requestBody["indexWithinSlot"].(float64)
		if !ok || indexWithinSlot < 0 || indexWithinSlot > 5 {
			funcs.Log(services.ERROR, "Invalid indexWithinSlot", "grag")
			funcs.CreateError(
				"errors.com.epicgames.validation.validation_failed",
				"Invalid indexWithinSlot",
				[]string{},
				1040,
				"any",
				"prod",
				"Invalid indexWithinSlot",
				"validation_failed",
				w,
			)
			return
		}

		lockerSlotsData := profileData["items"].(map[string]interface{})[activeLoadoutId].(map[string]interface{})["attributes"].(map[string]interface{})["locker_slots_data"].(map[string]interface{})["slots"].(map[string]interface{})[slotName].(map[string]interface{})
		items, ok := lockerSlotsData["items"].(primitive.A)
		if !ok {
			funcs.Log(services.ERROR, "Invalid lockerSlotsData items", "poggers")
			funcs.CreateError(
				"errors.com.epicgames.validation.validation_failed",
				"Invalid lockerSlotsData items",
				[]string{},
				1040,
				"any",
				"prod",
				"Invalid lockerSlotsData items",
				"validation_failed",
				w,
			)
			return
		}

		if int(indexWithinSlot) >= len(items) {
			funcs.Log(services.ERROR, "Index out of range", "wow")
			funcs.CreateError(
				"errors.com.epicgames.validation.validation_failed",
				"Index out of range",
				[]string{},
				1040,
				"any",
				"prod",
				"Index out of range",
				"validation_failed",
				w,
			)
			return
		}

		items[int(indexWithinSlot)] = templateId
		lockerSlotsData["items"] = items

		attributes, ok := profileData["stats"].(map[string]interface{})["attributes"].(map[string]interface{})
		if !ok {
			funcs.Log(services.ERROR, "Invalid profile attributes", "fuck")
			funcs.CreateError(
				"errors.com.epicgames.validation.validation_failed",
				"Invalid profile attributes",
				[]string{},
				1040,
				"any",
				"prod",
				"Invalid profile attributes",
				"validation_failed",
				w,
			)
			return
		}

		favoriteDance, ok := attributes["favorite_dance"].(primitive.A)
		if !ok {
			funcs.Log(services.ERROR, "Invalid favorite_dance data", "ok")
			funcs.CreateError(
				"errors.com.epicgames.validation.validation_failed",
				"Invalid favorite_dance data",
				[]string{},
				1040,
				"any",
				"prod",
				"Invalid favorite_dance data",
				"validation_failed",
				w,
			)
			return
		}
		favoriteDance[int(indexWithinSlot)] = itemToSlot

		applyProfileChanges = append(applyProfileChanges, map[string]interface{}{
			"changeType": "statModified",
			"name":       "favorite_dance",
			"value":      favoriteDance,
		})

	default:
		if slotName == "Pickaxe" || slotName == "Glider" {
			if itemToSlot == "" {
				funcs.Log(services.ERROR, "Missing itemToSlot for slot", slotName)
				funcs.CreateError(
					"errors.com.epicgames.validation.validation_failed",
					"Missing itemToSlot",
					[]string{},
					1040,
					"any",
					"prod",
					"Missing itemToSlot",
					"validation_failed",
					w,
				)
				return
			}
		}

		favoriteSlot := strings.ToLower("favorite_" + slotName)
		profileData["stats"].(map[string]interface{})["attributes"].(map[string]interface{})[favoriteSlot] = itemToSlot

		lockerSlotsData := profileData["items"].(map[string]interface{})[activeLoadoutId].(map[string]interface{})["attributes"].(map[string]interface{})["locker_slots_data"].(map[string]interface{})["slots"].(map[string]interface{})[slotName].(map[string]interface{})
		lockerSlotsData["items"] = []interface{}{templateId}

		applyProfileChanges = append(applyProfileChanges, map[string]interface{}{
			"changeType": "statModified",
			"name":       favoriteSlot,
			"value":      itemToSlot,
		})
	}

	existingProfile.ProfileData[profileId] = profileData
	_, err = profilesCollection.ReplaceOne(context.Background(), bson.M{"accountid": accountId}, existingProfile)
	if err != nil {
		funcs.Log(services.ERROR, "Failed to update profile", err.Error())
		funcs.CreateError(
			"errors.com.epicgames.common.internal_error",
			"Failed to update profile",
			[]string{},
			1000,
			"any",
			"prod",
			"Failed to update profile",
			"internal_error",
			w,
		)
		return
	}

	response := map[string]interface{}{
		"profileRevision":            profileData["rvn"],
		"profileId":                  profileId,
		"profileChangesBaseRevision": rvn,
		"profileChanges":             applyProfileChanges,
		"profileCommandRevision":     profileData["commandRevision"],
		"serverTime":                 time.Now().UTC().Format(time.RFC3339),
		"responseVersion":            1,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		funcs.Log(services.ERROR, "Failed to encode response", err.Error())
		funcs.CreateError(
			"errors.com.epicgames.common.internal_error",
			"Failed to encode response",
			[]string{},
			1000,
			"any",
			"prod",
			"Failed to encode response",
			"internal_error",
			w,
		)
	}
}
