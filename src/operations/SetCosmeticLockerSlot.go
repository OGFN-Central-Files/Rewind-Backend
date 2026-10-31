package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go-backend/src/database/models"
	"go-backend/src/global"
	"go-backend/src/services"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (f *Operations) SetCosmeticLockerSlot(w http.ResponseWriter, r *http.Request) {
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

	itemToSlot, ok := requestBody["itemToSlot"].(string)
	if !ok {
		itemToSlot = ""
	}

	variantsUpdates, ok := requestBody["variantUpdates"].([]interface{})
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

		itemsMap, ok := profileData["items"].(map[string]interface{})
		if !ok {

		} else {
			itemData, ok := itemsMap[itemToSlot].(map[string]interface{})
			if !ok {
			} else {
				attributes, ok := itemData["attributes"].(map[string]interface{})
				if !ok {
				} else {
					variants, ok := attributes["variants"].([]interface{})

					if !ok {
						attributes["variants"] = []interface{}{}
						itemData["attributes"] = attributes
						itemsMap[itemToSlot] = itemData
						profileData["items"] = itemsMap

						variants = []interface{}{}
					}

					if len(variants) == 0 {
						partsVariant := map[string]interface{}{
							"channel": "Parts",
							"active":  "Stage1",
							"owned":   []interface{}{},
						}
						materialVariant := map[string]interface{}{
							"channel": "Material",
							"active":  "Mat2",
							"owned":   []interface{}{},
						}
						variants = append(variants, partsVariant, materialVariant)
						attributes["variants"] = variants
						itemData["attributes"] = attributes
						itemsMap[itemToSlot] = itemData
						profileData["items"] = itemsMap
					}

					for _, v := range variantsUpdates {
						updateMap, ok := v.(map[string]interface{})
						if !ok {
							continue
						}

						channel, okChan := updateMap["channel"].(string)
						active, okActive := updateMap["active"].(string)
						if !okChan || !okActive || channel == "" || active == "" {
							continue
						}

						foundIndex := -1
						for idx, variant := range variants {
							vMap, ok := variant.(map[string]interface{})
							if !ok {
								continue
							}
							if vMap["channel"] == channel {
								foundIndex = idx
								break
							}
						}

						if foundIndex == -1 {
							continue
						}

						variantMap, ok := variants[foundIndex].(map[string]interface{})
						if !ok {
							continue
						}
						variantMap["active"] = active
						variants[foundIndex] = variantMap
					}

					attributes["variants"] = variants
					itemData["attributes"] = attributes
					itemsMap[itemToSlot] = itemData
					profileData["items"] = itemsMap

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

	slotName := requestBody["category"].(string)
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
		indexWithinSlot, ok := requestBody["slotIndex"].(float64)
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

	case "ItemWrap":
		indexWithinSlot, ok := requestBody["slotIndex"].(float64)
		if !ok || int(indexWithinSlot) < -1 || int(indexWithinSlot) > 7 {
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

		lockerSlotsData, ok := profileData["items"].(map[string]interface{})[activeLoadoutId].(map[string]interface{})["attributes"].(map[string]interface{})["locker_slots_data"].(map[string]interface{})["slots"].(map[string]interface{})[slotName].(map[string]interface{})
		if !ok {
			funcs.Log(services.ERROR, "Invalid lockerSlotsData", "poggers")
			funcs.CreateError(
				"errors.com.epicgames.validation.validation_failed",
				"Invalid lockerSlotsData",
				[]string{},
				1040,
				"any",
				"prod",
				"Invalid lockerSlotsData",
				"validation_failed",
				w,
			)
			return
		}

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

		favoriteItemWraps, ok := attributes["favorite_itemwraps"].(primitive.A)
		if !ok {
			funcs.Log(services.ERROR, "Invalid favorite_itemwraps data", "ok")
			funcs.CreateError(
				"errors.com.epicgames.validation.validation_failed",
				"Invalid favorite_itemwraps data",
				[]string{},
				1040,
				"any",
				"prod",
				"Invalid favorite_itemwraps data",
				"validation_failed",
				w,
			)
			return
		}

		if int(indexWithinSlot) == -1 {
			for i := 0; i < 7; i++ {
				if i < len(items) {
					items[i] = templateId
				}
				if i < len(favoriteItemWraps) {
					favoriteItemWraps[i] = templateId
				}
			}
		} else {
			idx := int(indexWithinSlot)
			if idx >= len(items) {
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
			items[idx] = templateId
			if idx < len(favoriteItemWraps) {
				favoriteItemWraps[idx] = templateId
			}
		}

		lockerSlotsData["items"] = items
		attributes["favorite_itemwraps"] = favoriteItemWraps

		applyProfileChanges = append(applyProfileChanges, map[string]interface{}{
			"changeType":     "itemAttrChanged",
			"itemId":         activeLoadoutId,
			"attributeName":  "locker_slots_data",
			"attributeValue": profileData["items"].(map[string]interface{})[activeLoadoutId].(map[string]interface{})["attributes"].(map[string]interface{})["locker_slots_data"],
		})

		applyProfileChanges = append(applyProfileChanges, map[string]interface{}{
			"changeType": "statModified",
			"name":       "favorite_itemwraps",
			"value":      favoriteItemWraps,
		})
	default:
		sandboxLoadout, ok := profileData["items"].(map[string]interface{})["sandbox_loadout"]
		if !ok {
			funcs.Log(services.ERROR, "Invalid sandbox_loadout", "sandbox_loadout is missing or invalid")
			funcs.CreateError(
				"errors.com.epicgames.validation.validation_failed",
				"Invalid sandbox_loadout",
				[]string{},
				1040,
				"any",
				"prod",
				"Invalid sandbox_loadout",
				"validation_failed",
				w,
			)
			return
		}

		loadout, ok := sandboxLoadout.(map[string]interface{})
		if !ok {
			funcs.Log(services.ERROR, "Invalid loadout format", fmt.Sprintf("Expected map[string]interface{}, but got: %T", sandboxLoadout))
			funcs.CreateError(
				"errors.com.epicgames.validation.validation_failed",
				fmt.Sprintf("Invalid loadout format: Expected map[string]interface{}, but got: %T", sandboxLoadout),
				[]string{},
				1040,
				"any",
				"prod",
				"Invalid loadout format",
				"validation_failed",
				w,
			)
			return
		}

		lockerSlotsDataInterface, ok := loadout["attributes"].(map[string]interface{})["locker_slots_data"].(map[string]interface{})
		if !ok {
			funcs.Log(services.ERROR, "Invalid locker slots data", "locker slots data is missing or invalid")
			funcs.CreateError(
				"errors.com.epicgames.validation.validation_failed",
				"Invalid locker slots data",
				[]string{},
				1040,
				"any",
				"prod",
				"Invalid locker slots data",
				"validation_failed",
				w,
			)
			return
		}

		slotsInterface, ok := lockerSlotsDataInterface["slots"].(map[string]interface{})
		if !ok {
			funcs.Log(services.ERROR, "Invalid slots data", "slots data is missing or invalid")
			funcs.CreateError(
				"errors.com.epicgames.validation.validation_failed",
				"Invalid slots data",
				[]string{},
				1040,
				"any",
				"prod",
				"Invalid slots data",
				"validation_failed",
				w,
			)
			return
		}

		backendType := slotName
		slotData, ok := slotsInterface[backendType].(map[string]interface{})
		if !ok {
			funcs.Log(services.ERROR, "Invalid backendType slot", fmt.Sprintf("Slot with backendType '%s' is missing or invalid", backendType))
			funcs.CreateError(
				"errors.com.epicgames.validation.validation_failed",
				fmt.Sprintf("Invalid slot data for backendType '%s'", backendType),
				[]string{},
				1040,
				"any",
				"prod",
				fmt.Sprintf("Invalid slot data for backendType '%s'", backendType),
				"validation_failed",
				w,
			)
			return
		}

		items := []interface{}{} // hlep idk why it breaks when i dont do this
		slotData["items"] = append(items, itemToSlot)
		applyProfileChanges = append(applyProfileChanges, map[string]interface{}{
			"changeType":     "itemAttrChanged",
			"itemId":         itemToSlot,
			"attributeName":  "locker_slots_data",
			"attributeValue": lockerSlotsDataInterface,
		})

		if backendType == "AthenaCharacter" || backendType == "Character" {
			cleanedSkin := funcs.SanitizeItemToSlot(itemToSlot)
			funcs.SetFavoriteSkin(existingProfile, cleanedSkin)
		}
	}

	athenarev3, _ := funcs.GetAndMultiplyRVN(profileData, "rvn", 2)
	athenacommandrev3, _ := funcs.GetAndMultiplyRVN(profileData, "commandRevision", 2)
	profileData["rvn"] = athenarev3
	profileData["commandRevision"] = athenacommandrev3
	profileData["updatedAt"] = time.Now().Format(time.RFC3339)

	key := fmt.Sprintf("profiledata.%s", profileId)
	_, err = profilesCollection.UpdateOne(
		context.Background(),
		bson.M{"accountid": existingAccount.AccountID},
		bson.M{"$set": bson.M{
			key: profileData,
		}},
	)
	if err != nil {
		funcs.Log(services.ERROR, "Failed to update profile in database", err.Error())
		funcs.CreateError(
			"errors.com.epicgames.common.internal_error",
			"Failed to update profile in database",
			[]string{},
			1000,
			"any",
			"prod",
			"Failed to update profile in database",
			"internal_error",
			w,
		)
		return
	}

	// NextProfileRVN2, _ := funcs.GetAndMultiplyRVN(profileData, "rvn", 1)
	// NextProfileCommandRVN2, _ := funcs.GetAndMultiplyRVN(profileData, "commandRevision", 1)

	applyProfileChanges = []map[string]interface{}{
		{
			"changeType": "fullProfileUpdate",
			"profile":    profileData,
		},
	}

	response := map[string]interface{}{
		"profileRevision":            NextProfileRVN,
		"profileId":                  profileId,
		"profileChangesBaseRevision": rvn,
		"profileChanges":             applyProfileChanges,
		"profileCommandRevision":     NextProfileCommandRVN,
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

func checkFields(fields []string, body map[string]interface{}) []string {
	missingFields := []string{}
	for _, field := range fields {
		if _, exists := body[field]; !exists {
			missingFields = append(missingFields, field)
		}
	}
	return missingFields
}
