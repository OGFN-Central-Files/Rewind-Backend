package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"go-backend/src/database/models"
	"go-backend/src/global"
	"go-backend/src/services"
	"net/http"
	"os"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func InitFortnite() {
	http.HandleFunc("/fortnite/api/game/v2/enabled_features", EnabledFeatures)
	http.HandleFunc("/fortnite/api/game/v2/grant_access/{accountId}", GrantAccess)
	http.HandleFunc("/content/api/pages/{pg}", ContentPages)
	http.HandleFunc("/content/api/pages/{pg}/", ContentPages)
	http.HandleFunc("/fortnite/api/receipts/v1/account/{accountId}/receipts", ReturnNull)
	http.HandleFunc("/fortnite/api/storefront/v2/catalog", Catalog)
	http.HandleFunc("/catalog/api/shared/bulk/offers", BulkOffers)
	http.HandleFunc("/waitingroom/api/waitingroom", WaitingRoom)
	http.HandleFunc("/eulatracking/api/public/agreements/fn/account/{accountId}", VerifyEULA)
	http.HandleFunc("/socialban/api/public/v1/{accountId}", SocialBan)
	http.HandleFunc("/fortnite/api/game/v2/br-inventory/account/{accountId}", BR_Inventory)
	http.HandleFunc("/region", RegionReq)
	http.HandleFunc("/sdk/v1/", HandleSdkV1)
	http.HandleFunc("/fortnite/api/storefront/v2/gift/check_eligibility/recipient/{recipientId}/offer/{offerId}", GiftCheckEligibility)
}

func GiftCheckEligibility(w http.ResponseWriter, r *http.Request) {
	segments := strings.Split(r.URL.Path, "/")

	if len(segments) < 10 || segments[7] != "recipient" || segments[9] != "offer" {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	recipientId := segments[8]
	offerId := segments[10]

	offer, err := funcs.FindOfferId(global.Catalog, offerId)
	if err != nil || offer == nil {
		funcs.CreateErrorV2(
			"errors.com.epicgames.events.download_failed",
			"Failed to read events data",
			[]string{offerId}, -1, "", 500, w,
		)
		return
	}

	tokenManager := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenManager.VerifyToken(w, r)
	if user == nil {
		return
	}

	isFriend := false

	if friendsRaw, ok := user.Additional["friends"]; ok {
		if friendsList, ok := friendsRaw.(primitive.A); ok {
			for _, entry := range friendsList {
				if friend, ok := entry.(map[string]interface{}); ok {
					accountId, ok := friend["accountId"].(string)
					if !ok {
						continue
					}

					if accountId == recipientId {
						isFriend = true
						break
					}
				}
			}
		}
	}

	if !isFriend && recipientId != user.AccountID {
		funcs.CreateErrorV2(
			"errors.com.epicgames.friends.no_relationship",
			fmt.Sprintf("User %s is not friends with %s", user.AccountID, recipientId),
			[]string{user.AccountID, recipientId},
			28004, "", 403, w,
		)
		return
	}

	profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")
	var existingProfile *models.Profile
	err = profilesCollection.FindOne(context.Background(), bson.M{"accountid": user.AccountID}).Decode(&existingProfile)
	if err != nil {
		funcs.CreateErrorV2(
			"errors.com.epicgames.oauth.no_profile",
			fmt.Sprintf("%s doesn't have a valid profile", user.AccountID),
			[]string{},
			28004, "", 403, w,
		)
		return
	}

	var receivingProfile *models.Profile
	err = profilesCollection.FindOne(context.Background(), bson.M{"accountid": recipientId}).Decode(&receivingProfile)
	if err != nil {
		funcs.CreateErrorV2(
			"errors.com.epicgames.oauth.no_profile",
			fmt.Sprintf("%s doesn't have a valid profile", recipientId),
			[]string{},
			28004, "", 403, w,
		)
		return
	}

	athena := receivingProfile.ProfileData["athena"].(map[string]interface{})
	itemsMap, ok := athena["items"].(map[string]interface{})
	if !ok {
		funcs.CreateErrorV2(
			"errors.com.epicgames.profile.invalid_data",
			"Invalid profile data format",
			nil, 28004, "", 500, w,
		)
		return
	}

	for _, itemGrant := range offer.ItemGrants {
		itemTemplateID := strings.ToLower(itemGrant.TemplateID)

		for _, item := range itemsMap {
			if itemData, ok := item.(map[string]interface{}); ok {
				if userItemTemplateID, ok := itemData["templateId"].(string); ok &&
					strings.ToLower(userItemTemplateID) == itemTemplateID {
					funcs.CreateErrorV2(
						"errors.com.epicgames.modules.gamesubcatalog.purchase_not_allowed",
						fmt.Sprintf("Could not purchase catalog offer %s, item %s", offer.Name, itemGrant.TemplateID),
						[]string{offer.Name, itemGrant.TemplateID},
						28004, "", 403, w,
					)
					return
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"price": offer.Prices[0],
		"items": offer.ItemGrants,
	}

	json.NewEncoder(w).Encode(response)
}

func HandleSdkV1(w http.ResponseWriter, r *http.Request) {
	filePath := "./static/responses/sdkv1.json"
	contentBytes, err := os.ReadFile(filePath)

	funcs := &services.Functions{}

	if err != nil {
		funcs.CreateErrorV2(
			"errors.com.epicgames.account.sdkv1.failed",
			"internal Server Error",
			[]string{}, -1, "", 400, w,
		)
		return
	}

	var bulkOffers map[string]interface{}
	err = json.Unmarshal(contentBytes, &bulkOffers)
	if err != nil {
		funcs.CreateErrorV2(
			"errors.com.epicgames.account.sdkv1.failed",
			"Failed to unmarshal bytes (sdkv1.json)",
			[]string{}, -1, "", 400, w,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(bulkOffers); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}

func BR_Inventory(w http.ResponseWriter, r *http.Request) {

	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenCreation.VerifyToken(w, r)
	if user == nil {
		return
	}

	response := map[string]interface{}{
		"stash": map[string]interface{}{
			"globalcash": 0,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func RegionReq(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"continent": map[string]interface{}{
			"code":       "EU",
			"geoname_id": 6255148,
			"names": map[string]interface{}{
				"de":    "Europa",
				"en":    "Europe",
				"es":    "Europa",
				"fr":    "Europe",
				"ja":    "ヨーロッパ",
				"pt-BR": "Europa",
				"ru":    "Европа",
				"zh-CN": "欧洲",
			},
		},
		"country": map[string]interface{}{
			"geoname_id":           2635167,
			"is_in_european_union": false,
			"iso_code":             "GB",
			"names": map[string]interface{}{
				"de":    "UK",
				"en":    "United Kingdom",
				"es":    "RU",
				"fr":    "Royaume Uni",
				"ja":    "英国",
				"pt-BR": "Reino Unido",
				"ru":    "Британия",
				"zh-CN": "英国",
			},
		},
		"subdivisions": []map[string]interface{}{
			{
				"geoname_id": 6269131,
				"iso_code":   "ENG",
				"names": map[string]interface{}{
					"de":    "England",
					"en":    "England",
					"es":    "Inglaterra",
					"fr":    "Angleterre",
					"ja":    "イングランド",
					"pt-BR": "Inglaterra",
					"ru":    "Англия",
					"zh-CN": "英格兰",
				},
			},
			{
				"geoname_id": 3333157,
				"iso_code":   "KEC",
				"names": map[string]interface{}{
					"en": "Royal Kensington and Chelsea",
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func SocialBan(w http.ResponseWriter, r *http.Request) {
	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenCreation.VerifyToken(w, r)
	if user == nil {
		return
	}

	response := map[string]interface{}{
		"bans": []interface{}{
			// map[string]interface{}{
			// 	"BanReasons": []interface{}{
			// 		4,
			// 	},
			// 	"ExploitProgramName":   "cheat",
			// 	"AdditionalInfo":       "",
			// 	"BanStartTime":         time.Now().Format(time.RFC3339),
			// 	"BanDurationDays":      3,
			// 	"CompetitiveBanReason": 1,
			// 	"bRequiresUserAck":     true,
			// 	"bBanHasStart":         true,
			// },
		},
		"warnings": []interface{}{},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func ContentPages(w http.ResponseWriter, r *http.Request) {
	funcs := &services.Functions{}
	contentPages, err := funcs.GetContentPages(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting content pages: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(contentPages); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}

func Catalog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(global.Catalog); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}

func BulkOffers(w http.ResponseWriter, r *http.Request) {
	filePath := "./static/responses/bulkOffers.json"
	contentBytes, err := os.ReadFile(filePath)

	funcs := &services.Functions{}

	if err != nil {
		funcs.CreateErrorV2(
			"errors.com.epicgames.account.bulkoffers.failed",
			"internal Server Error",
			[]string{}, -1, "", 400, w,
		)
		return
	}

	var bulkOffers map[string]interface{}
	err = json.Unmarshal(contentBytes, &bulkOffers)
	if err != nil {
		funcs.CreateErrorV2(
			"errors.com.epicgames.account.bulkoffers.failed",
			"Failed to unmarshal bytes (bulkOffers.json)",
			[]string{}, -1, "", 400, w,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(bulkOffers); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}

func ReturnNull(w http.ResponseWriter, r *http.Request) {
	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenCreation.VerifyToken(w, r)
	if user == nil {
		return
	}

	response := `[]`
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(response))
}

func EnabledFeatures(w http.ResponseWriter, r *http.Request) {
	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenCreation.VerifyToken(w, r)
	if user == nil {
		return
	}

	response := `[]`
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(response))
}

func GrantAccess(w http.ResponseWriter, r *http.Request) {
	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenCreation.VerifyToken(w, r)
	if user == nil {
		return
	}

	response := `{}`
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(response))
}

func WaitingRoom(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	ticket := query.Get("ticket")
	if ticket != "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	ticketId := "gyat"
	time := 0

	response := map[string]interface{}{
		"ticketId":     ticketId,
		"expectedWait": time,
		"retryTime":    time,
		"throttled":    false,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func VerifyEULA(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
