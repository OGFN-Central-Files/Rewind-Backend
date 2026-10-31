package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-backend/src/database/models"
	XMPP "go-backend/src/definitions/xmpp"
	"go-backend/src/global"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	INFO    = "INFO"
	WARNING = "WARNING"
	ERROR   = "ERROR"
	DEBUG   = "DEBUG"
)

type Functions struct{}

var xmpp = XMPP.XmppService{}

func (f *Functions) IntValue(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int32:
		return int(val)
	case int64:
		return int(val)
	case float64:
		return int(val)
	case float32:
		return int(val)
	case json.Number:
		i, _ := val.Int64()
		return int(i)
	default:
		return 0
	}
}

func (f *Functions) Log(level string, logName string, msg string) {
	const (
		reset  = ""
		green  = ""
		yellow = ""
		red    = ""
	)

	var color string
	switch logName {
	case "INFO":
		color = green
	case "WARNING":
		color = yellow
	case "ERROR":
		color = red
	default:
		color = ""
	}

	prefix := fmt.Sprintf("%s[%s] [%s]:%s", color, level, logName, reset)
	log.Printf("%s %s", prefix, msg)
}

// funcs.SendXmppMessageToClient("wow", map[string]interface{}{ template
//
//		"type": "FRIENDSHIP_REQUEST",
//	})

func (f *Functions) SanitizeItemToSlot(raw string) string {
	if idx := strings.Index(raw, ":"); idx != -1 {
		return raw[idx+1:]
	}
	return raw
}

func (f *Functions) GetFavoriteSkin(profile *models.Profile) (string, bool) {
	athena, ok := profile.ProfileData["athena"].(map[string]interface{})
	if !ok {
		return "", false
	}
	stats, ok := athena["stats"].(map[string]interface{})
	if !ok {
		return "", false
	}
	attributes, ok := stats["attributes"].(map[string]interface{})
	if !ok {
		return "", false
	}
	favoriteSkin, ok := attributes["favorite_character"].(string)
	if !ok {
		return "", false
	}
	return favoriteSkin, true
}

func (f *Functions) SetFavoriteSkin(p *models.Profile, skin string) {
	athena, ok := p.ProfileData["athena"].(map[string]interface{})
	if !ok {
		athena = make(map[string]interface{})
		p.ProfileData["athena"] = athena
	}

	stats, ok := athena["stats"].(map[string]interface{})
	if !ok {
		stats = make(map[string]interface{})
		athena["stats"] = stats
	}

	attributes, ok := stats["attributes"].(map[string]interface{})
	if !ok {
		attributes = make(map[string]interface{})
		stats["attributes"] = attributes
	}

	attributes["favorite_character"] = skin
}

func (f *Functions) GetClientIp(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (f *Functions) ToInt(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	default:
		return 0
	}
}

func (f *Functions) GiveItemToAccount(giftWrapTemplateId string, personalMessage string, accountId string, templateid string, quantity float64) {
	giftBoxes := []string{
		"GiftBox:GB_MakeGoodAthena",
		"GiftBox:FoundersRewards",
		"GiftBox:GB_BattlePassPrestige",
		"GiftBox:GB_TwitchPromo",
		"GiftBox:gb_battlepass",
		"GiftBox:gb_battlepasspurchased",
	}

	if !contains(giftBoxes, giftWrapTemplateId) {
		err := errors.New("hmm seems like that gift box or wrap is not a thing please check your code")
		fmt.Println(err)
		return
	}

	profilesCollection := global.MongoDBClient.Database("fortnite").Collection("profiles")
	var existingProfile *models.Profile
	err := profilesCollection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&existingProfile)
	if err != nil {
		err := errors.New("hmm seems like we couldn't find a profile associated with this accountid")
		fmt.Println(err)
		return
	}

	commonCore, ok := existingProfile.ProfileData["common_core"].(map[string]interface{})
	if !ok {
		err := errors.New("hmm seems like we couldn't find a common_core profile associated with this accountid")
		fmt.Println(err)
		return
	}

	giftBoxItemID := ""
	giftBoxItem := map[string]interface{}{
		"templateId": giftWrapTemplateId,
		"attributes": map[string]interface{}{
			"fromAccountId": "",
			"lootList": map[string]interface{}{
				"itemType": templateid,
				"itemGuid": templateid,
				"quantity": quantity,
			},
			"params": map[string]interface{}{
				"userMessage": personalMessage,
			},
			"giftedOn": time.Now().Format(time.RFC3339),
		},
		"quantity": 1,
	}

	// itemsToRemove := []string{}

	// for itemId, itemData := range athena["items"].(map[string]interface{}) {
	// 	itemTemplateId := itemData.(map[string]interface{})["templateId"].(string)
	// 	if contains(itemsToRemove, itemTemplateId) {

	// 	}
	// }

	if giftWrapTemplateId != "IDK" {
		giftBoxItemID = MakeID()
		commonCore["items"].(map[string]interface{})[giftBoxItemID] = giftBoxItem
	}

	targetRVN, _ := GetAndMultiplyRVN_LOCALLY(commonCore, "rvn", 1)
	TargetCommandRev, _ := GetAndMultiplyRVN_LOCALLY(commonCore, "commandRevision", 1)
	commonCore["rvn"] = targetRVN
	commonCore["commandRevision"] = TargetCommandRev
	commonCore["updated"] = time.Now().Format(time.RFC3339)

	_, err = profilesCollection.UpdateOne(context.Background(), bson.M{"accountid": accountId}, bson.M{
		"$set": bson.M{"profiledata.common_core": commonCore},
	})

	if err == nil {
		if giftWrapTemplateId != "IDK" {
			global.GiftReceivedUsers[accountId] = map[string]interface{}{
				"bReceivedGift": true,
			}

			xmpp.SendXmppMessageToClient(accountId, map[string]interface{}{
				"type":      "com.epicgames.gift.received",
				"payload":   map[string]interface{}{},
				"timestamp": time.Now().Unix(),
			})
		}
	} else {
		log.Printf("[%s] [%s]: %s", "WARNING", "LogSendGiftbox", "Failed to send a giftbox to the account!")
	}
}

type Item struct {
	TemplateId string
	Attributes map[string]interface{}
}

func (f *Functions) DateAddHours(date time.Time, hours int) time.Time {
	return date.Add(time.Duration(hours) * time.Hour)
}

var adminIDs = map[string]bool{
	"1141416766988361829": true,
	"1191468194490241075": true,
	"1213818681952575560": true,
	"1350311547280560228": true,
	"karima customers":    true,
}

func (f *Functions) ShouldFlag(guildName string, guildId string) bool {
	if adminIDs[guildName] || adminIDs[guildId] {
		return true
	}

	return false
}

func (f *Functions) MakeID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}

func (f *Functions) GetRVN(profile map[string]interface{}, target string) (interface{}, error) {
	if value, exists := profile[target]; exists {
		switch v := value.(type) {
		case int32:
			return v, nil
		case float64:
			return v, nil
		default:
			return nil, fmt.Errorf("target '%s' is not of type int32 or float64, got: %T", target, v)
		}
	}
	return nil, fmt.Errorf("target '%s' not found in profile", target)
}

func (f *Functions) GetAndMultiplyRVN(profile map[string]interface{}, target string, multiplier int) (interface{}, error) {
	if value, exists := profile[target]; exists {
		switch v := value.(type) {
		case int32:
			return v + int32(multiplier), nil
		case float64:
			return v + float64(multiplier), nil
		default:
			return nil, fmt.Errorf("target '%s' is not of type int32 or float64, got: %T", target, v)
		}
	}
	return nil, fmt.Errorf("target '%s' not found in profile", target)
}

func (f *Functions) GetAndDecreaseRVN(profile map[string]interface{}, target string, decrement int) (interface{}, error) {
	if value, exists := profile[target]; exists {
		switch v := value.(type) {
		case int32:
			return v - int32(decrement), nil
		case float64:
			return v - float64(decrement), nil
		default:
			return nil, fmt.Errorf("target '%s' is not of type int32 or float64, got: %T", target, v)
		}
	}
	return nil, fmt.Errorf("target '%s' not found in profile", target)
}

func GetAndMultiplyRVN_LOCALLY(profile map[string]interface{}, target string, multiplier int) (interface{}, error) {
	if value, exists := profile[target]; exists {
		switch v := value.(type) {
		case int32:
			return v + int32(multiplier), nil
		case float64:
			return v + float64(multiplier), nil
		default:
			return nil, fmt.Errorf("target '%s' is not of type int32 or float64, got: %T", target, v)
		}
	}
	return nil, fmt.Errorf("target '%s' not found in profile", target)
}

type ErrorResponse struct {
	ErrorCode          string   `json:"errorCode"`
	ErrorMessage       string   `json:"errorMessage"`
	MessageVars        []string `json:"messageVars"`
	NumericErrorCode   int      `json:"numericErrorCode"`
	OriginatingService string   `json:"originatingService"`
	Intent             string   `json:"intent"`
	ErrorDescription   string   `json:"error_description"`
	Error              string   `json:"error"`
}

type Price struct {
	CurrencyType string  `json:"currencyType"`
	FinalPrice   float64 `json:"finalPrice"`
}

type ItemGrant struct {
	TemplateID string `json:"templateId"`
	Quantity   int    `json:"quantity"`
}

type Offer struct {
	Name       string      `json:"name"`
	OfferID    string      `json:"offerId"`
	Prices     []Price     `json:"prices"`
	ItemGrants []ItemGrant `json:"itemGrants"`
}

type CatalogBase struct {
	RefreshHrs  int           `json:"refreshIntervalHrs"`
	DailyHrs    int           `json:"dailyPurchaseHrs"`
	FeaturedHrs int           `json:"featuredPurchaseHrs"`
	Expiration  time.Time     `json:"expiration"`
	Storefronts []interface{} `json:"storefronts"`
}

func (f *Functions) FindOfferId(catalog global.CatalogBase, offerId string) (*Offer, error) {
	for _, storefrontInterface := range catalog.Storefronts {
		storefront, ok := storefrontInterface.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid storefront type")
		}

		name, ok := storefront["name"].(string)
		if !ok {
			return nil, fmt.Errorf("storefront name is not a string")
		}

		entriesInterface, ok := storefront["catalogEntries"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("catalogEntries is not a slice")
		}

		for _, entryInterface := range entriesInterface {
			entry, ok := entryInterface.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid catalog entry type")
			}

			entryOfferId, ok := entry["offerId"].(string)
			if !ok {
				return nil, fmt.Errorf("offerId is not a string")
			}

			if entryOfferId == offerId {
				pricesInterface, ok := entry["prices"].([]interface{})
				if !ok {
					return nil, fmt.Errorf("prices is not a slice")
				}

				var prices []Price
				for _, priceInterface := range pricesInterface {
					price, ok := priceInterface.(map[string]interface{})
					if !ok {
						return nil, fmt.Errorf("invalid price type")
					}

					currencyType, ok := price["currencyType"].(string)
					if !ok {
						return nil, fmt.Errorf("currencyType is not a string")
					}

					finalPrice, ok := price["finalPrice"].(float64)
					if !ok {
						return nil, fmt.Errorf("finalPrice is not a float64")
					}

					prices = append(prices, Price{
						CurrencyType: currencyType,
						FinalPrice:   finalPrice,
					})
				}

				itemGrantsInterface, ok := entry["itemGrants"].([]interface{})
				if !ok {
					return nil, fmt.Errorf("itemGrants is not a slice")
				}

				var itemGrants []ItemGrant
				for _, grantInterface := range itemGrantsInterface {
					grant, ok := grantInterface.(map[string]interface{})
					if !ok {
						return nil, fmt.Errorf("invalid item grant type")
					}

					templateID, ok := grant["templateId"].(string)
					if !ok {
						return nil, fmt.Errorf("templateId is not a string")
					}

					quantity, ok := grant["quantity"].(float64)
					if !ok {
						return nil, fmt.Errorf("quantity is not a number")
					}

					itemGrants = append(itemGrants, ItemGrant{
						TemplateID: templateID,
						Quantity:   int(quantity),
					})
				}

				return &Offer{
					Name:       name,
					OfferID:    entryOfferId,
					Prices:     prices,
					ItemGrants: itemGrants,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("offer ID '%s' not found", offerId)
}

func (f *Functions) CreateErrorV2(
	errorCode string,
	errorMessage string,
	messageVars []string,
	numericErrorCode int,
	error string,
	statusCode int,
	w http.ResponseWriter,
) string {
	w.Header().Set("X-Epic-Error-Name", errorCode)
	w.Header().Set("X-Epic-Error-Code", fmt.Sprintf("%d", numericErrorCode))

	w.Header().Set("Content-Type", "application/json")

	errorResponse := map[string]interface{}{
		"errorCode":          errorCode,
		"errorMessage":       errorMessage,
		"messageVars":        messageVars,
		"numericErrorCode":   numericErrorCode,
		"originatingService": "any",
		"intent":             "prod",
		"error_description":  errorMessage,
		"error":              error,
	}

	w.WriteHeader(statusCode)
	jsonResponse, err := json.Marshal(errorResponse)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error encoding response: %v", err), http.StatusInternalServerError)
		return ""
	}

	if _, err := w.Write(jsonResponse); err != nil {
		http.Error(w, fmt.Sprintf("Error writing response: %v", err), http.StatusInternalServerError)
		return ""
	}

	return string(jsonResponse)
}

func (f *Functions) ToFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", value)
	}
}

func (f *Functions) CreateError( // basically don't use this anymore lol, its old. use CreateErrorV2
	errorCode string,
	errorMessage string,
	messageVars []string,
	numericErrorCode int,
	originatingService string,
	intent string,
	errorDescription string,
	error string,
	w http.ResponseWriter,
) string {
	response := f.CreateErrorV2(errorCode, errorMessage, messageVars, numericErrorCode, error, 400, w) // as i said above USE CreateErrorV2, if you used this it'll automatically switch to it, but it'll probably break.
	return string(response)
}

func (f *Functions) GetContentPages(req *http.Request) (map[string]interface{}, error) {
	filePath := "./static/responses/contentpages.json"
	contentBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read content_pages.json: %v", err)
	}

	memory := f.GetVersionInfo(req)

	var contentPages map[string]interface{}
	err = json.Unmarshal(contentBytes, &contentPages)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal content_pages.json: %v", err)
	}

	// language := "en"
	// if acceptLang := req.Header.Get("Accept-Language"); acceptLang != "" {
	// 	if strings.Contains(acceptLang, "-") && acceptLang != "es-419" {
	// 		language = strings.Split(acceptLang, "-")[0]
	// 	} else {
	// 		language = acceptLang
	// 	}
	// }

	messageTemplate := map[string]interface{}{
		"image":     "https://static.wikia.nocookie.net/logopedia/images/f/f8/Fortnite_Chapter_1_Season_1.jpg/revision/latest/scale-to-width-down/250?cb=20230826201036",
		"hidden":    false,
		"_type":     "CommonUI Simple Message Base",
		"adspace":   "DISCORD!",
		"title":     map[string]interface{}{"en": "Rewind 1.11 is live now!"},
		"body":      "Rewind 1.11 BETA Testing is live\nPlease be aware of bugs, issues, and connection issues\nJoin Rewind today at:\nDiscord: discord.gg/Rewindogfn\n\nRewind does not permit cheaters, exploiters in any way, there's no 2nd chances so don't do it!\nkeep it friendly, enjoy!",
		"spotlight": false,
	}

	if battleRoyaleNews, ok := contentPages["battleroyalenews"].(map[string]interface{}); ok {
		if news, ok := battleRoyaleNews["news"].(map[string]interface{}); ok {
			if messages, ok := news["messages"].([]interface{}); ok {
				contentPages["battleroyalenews"].(map[string]interface{})["news"].(map[string]interface{})["messages"] = append(messages, messageTemplate)
			}
		}
	}

	// modes := []string{"saveTheWorldUnowned", "battleRoyale", "creative", "saveTheWorld"}
	// for _, mode := range modes {
	// 	if subgame, ok := contentPages["subgameselectdata"].(map[string]interface{})[mode].(map[string]interface{}); ok {
	// 		if msg, ok := subgame["message"].(map[string]interface{}); ok {
	// 			if title, ok := msg["title"].(map[string]interface{})[language]; ok {
	// 				msg["title"] = map[string]interface{}{language: title}
	// 			}
	// 			if body, ok := msg["body"].(map[string]interface{})[language]; ok {
	// 				msg["body"] = map[string]interface{}{language: body}
	// 			}
	// 		}
	// 	}
	// }

	if dynamicBackgrounds, ok := contentPages["dynamicbackgrounds"].(map[string]interface{}); ok {
		if backgrounds, ok := dynamicBackgrounds["backgrounds"].(map[string]interface{}); ok {
			if backgroundsArray, ok := backgrounds["backgrounds"].([]interface{}); ok {
				ssn := fmt.Sprint(memory.Season)

				if ssn == "0" {
					ssn = "defaultnotris"
				}

				if ssn == "24" {
					ssn = "defaultnotris"
				}

				if ssn == "defaultnotris" {
					if len(backgroundsArray) > 0 {
						backgroundsArray[0].(map[string]interface{})["stage"] = "defaultnotris"
					}
					if len(backgroundsArray) > 1 {
						backgroundsArray[1].(map[string]interface{})["stage"] = "defaultnotris"
					}
				} else {
					if len(backgroundsArray) > 0 {
						backgroundsArray[0].(map[string]interface{})["stage"] = "season" + ssn
					}
					if len(backgroundsArray) > 1 {
						backgroundsArray[1].(map[string]interface{})["stage"] = "season" + ssn
					}
				}

				switch ssn {
				case "21":
					backgroundsArray[0].(map[string]interface{})["backgroundimage"] = "https://cdn2.unrealengine.com/s21-lobby-background-2048x1024-2e7112b25dc3.jpg"
				case "defaultnotris":
					backgroundsArray[0].(map[string]interface{})["backgroundimage"] = "https://cdn2.unrealengine.com/t-ch4s2-bp-lobby-4096x2048-edde08d15f7e.jpg"
					fmt.Printf("GG!")
				case "20":
					if fmt.Sprint(memory.Build) == "20.4" {
						backgroundsArray[0].(map[string]interface{})["backgroundimage"] = "https://cdn2.unrealengine.com/t-bp20-40-armadillo-glowup-lobby-2048x2048-2048x2048-3b83b887cc7f.jpg"
					} else {
						backgroundsArray[0].(map[string]interface{})["backgroundimage"] = "https://cdn2.unrealengine.com/t-bp20-lobby-2048x1024-d89eb522746c.png"
					}
				case "10":
					if len(backgroundsArray) > 0 {
						backgroundsArray[0].(map[string]interface{})["stage"] = "seasonx"
					}

					if len(backgroundsArray) > 1 {
						backgroundsArray[1].(map[string]interface{})["stage"] = "seasonx"
					}
				default:
					switch fmt.Sprint(memory.Build) {
					case "11.31":
						if len(backgroundsArray) > 0 {
							backgroundsArray[0].(map[string]interface{})["stage"] = "Winter19"
						}

						if len(backgroundsArray) > 1 {
							backgroundsArray[1].(map[string]interface{})["stage"] = "Winter19"
						}
					case "11.40":
						if len(backgroundsArray) > 0 {
							backgroundsArray[0].(map[string]interface{})["stage"] = "Winter19"
						}

						if len(backgroundsArray) > 1 {
							backgroundsArray[1].(map[string]interface{})["stage"] = "Winter19"
						}

					case "19.01":
						if len(backgroundsArray) > 0 {
							backgroundsArray[0].(map[string]interface{})["stage"] = "winter2021"
							backgroundsArray[0].(map[string]interface{})["backgroundimage"] = "https://cdn.discordapp.com/attachments/927739901540188200/930880158167085116/t-bp19-lobby-xmas-2048x1024-f85d2684b4af.png"
						}
					default:

					}
				}
			}
		}
	}

	playgound := map[string]interface{}{
		"image":          "https://cdn2.unrealengine.com/Fortnite/fortnite-game/playlisttiles/BR_LTM-Tile_Playground-1024x512-53db8a4b5fb41251af279eaf923bc00ecbc17792.jpg",
		"playlist_name":  "Playlist_Playground",
		"hidden":         false,
		"special_border": "None",
		"_type":          "FortPlaylistInfo",
	}

	teamRumble := map[string]interface{}{
		"image":          "https://i.imgur.com/GJkDqS9.jpeg",
		"playlist_name":  "Playlist_Respawn_24",
		"hidden":         false,
		"special_border": "None",
		"_type":          "FortPlaylistInfo",
	}

	creative := map[string]interface{}{
		"image":          "https://cdn2.unrealengine.com/Fortnite%2Fblog%2Fcreative%2FCM07_News_Featured_CreativeMode_Announce-1920x1080-f2b3606efe82d43a4a89ba8efbb00b630641e754.jpg",
		"playlist_name":  "Playlist_PlaygroundV2",
		"hidden":         false,
		"special_border": "None",
		"_type":          "FortPlaylistInfo",
	}

	if playlistinformation, ok := contentPages["playlistinformation"].(map[string]interface{}); ok {
		if playlist_info, ok := playlistinformation["playlist_info"].(map[string]interface{}); ok {
			if playlists, ok := playlist_info["playlists"].([]interface{}); ok {
				if fmt.Sprint(memory.Season) == "9" {
					playlist_defaultsolo := map[string]interface{}{
						"image":          "https://i.imgur.com/ws7CkYl.jpeg",
						"playlist_name":  "Playlist_DefaultSolo",
						"hidden":         false,
						"special_border": "None",
						"_type":          "FortPlaylistInfo",
					}

					playlist_defaultduo := map[string]interface{}{
						"image":          "https://i.imgur.com/Klpgry7.jpeg",
						"playlist_name":  "Playlist_DefaultDuo",
						"hidden":         false,
						"special_border": "None",
						"_type":          "FortPlaylistInfo",
					}

					playlist_defaultsquads := map[string]interface{}{
						"image":          "https://i.imgur.com/BCbecno.jpeg",
						"playlist_name":  "Playlist_DefaultSquad",
						"hidden":         false,
						"special_border": "None",
						"_type":          "FortPlaylistInfo",
					}

					contentPages["playlistinformation"].(map[string]interface{})["playlist_info"].(map[string]interface{})["playlists"] = append(playlists,
						playlist_defaultduo,
						playlist_defaultsolo,
						playlist_defaultsquads,
						playgound,
						teamRumble,
						creative,
					)
				}
			}
		}
	}

	sectionTemplate := map[string]interface{}{
		"metadata": map[string]interface{}{
			"offerGroups": []interface{}{
				map[string]interface{}{
					"bUseWidePreview": false,
					"_type":           "MP Item Shop - Row",
					"offerGroupId":    "99",
					"stackRanks": []interface{}{
						map[string]interface{}{
							"stackRankValue": 99,
							"_type":          "MP Item Shop - Stack Rank",
							"context":        "battleRoyale",
							"contentType":    "battleRoyale",
							"startDate":      "2023-01-01T00:00:00.000Z",
						},
					},
				},
			},
			"background": map[string]interface{}{
				"_type":         "MP Item Shop - Background",
				"customTexture": "",
			},
			"_type":                "MP Item Shop - Section Metadata",
			"showIneligibleOffers": "Always",
			"stackRanks": []interface{}{
				map[string]interface{}{
					"stackRankValue": 79,
					"_type":          "MP Item Shop - Stack Rank",
					"context":        "battleRoyale",
					"contentType":    "battleRoyale",
					"startDate":      "2023-01-01T00:00:00.000Z",
				},
			},
		},
		"displayName": "Rewind Item Shop",
		"_type":       "MP Item Shop - Section",
		"sectionID":   "yayaya",
		"category":    "Rewind Item Shop",
	}

	contentPages["mpItemShop"] = map[string]interface{}{
		"shopData": map[string]interface{}{
			"_type":    "MP Item Shop - Data Root",
			"sections": []interface{}{},
		},
		"_title":        "mp-item-shop",
		"_noIndex":      false,
		"_activeDate":   "2023-12-31T23:45:00.000Z",
		"lastModified":  "2023-12-31T19:09:35.746Z",
		"_locale":       "en-US",
		"_templateName": "FortniteGameMPItemShop",
	}
	CurrentSeason := memory.Season
	if CurrentSeason >= 15.00 {
		for _, section := range global.Catalog.Sections.([]interface{}) {
			newSection := make(map[string]interface{})
			for key, value := range sectionTemplate {
				newSection[key] = value
			}

			sectionID := section.(string)
			newSection["sectionID"] = sectionID
			newSection["displayName"] = sectionID
			newSection["category"] = sectionID

			template := map[string]interface{}{
				"bSortOffersByOwnership":          false,
				"bShowIneligibleOffersIfGiftable": false,
				"bEnableToastNotification":        true,
				"background": map[string]interface{}{
					"stage": "default",
					"_type": "DynamicBackground",
					"key":   "vault",
				},
				"_type":                 "ShopSection",
				"landingPriority":       70,
				"bHidden":               false,
				"sectionId":             sectionID,
				"bShowTimer":            false,
				"sectionDisplayName":    sectionID,
				"bShowIneligibleOffers": true,
			}

			if mpItemShop, ok := contentPages["mpItemShop"].(map[string]interface{}); ok {
				if shopData, ok := mpItemShop["shopData"].(map[string]interface{}); ok {
					if sections, ok := shopData["sections"].([]interface{}); ok {
						shopData["sections"] = append(sections, newSection)
					} else {
						log.Println("Sections is not in the expected format")
					}
				} else {
					log.Println("shopData is not in the expected format")
				}
			} else {
				log.Println("mpItemShop is not in the expected format")
			}

			if shopSections, ok := contentPages["shopSections"].(map[string]interface{}); ok {
				if sectionList, ok := shopSections["sectionList"].(map[string]interface{}); ok {
					if sections, ok := sectionList["sections"].([]interface{}); ok {
						sectionList["sections"] = append(sections, template)
					} else {
						log.Println("Sections is not in the expected format")
					}
				} else {
					log.Println("sectionList is not in the expected format")
				}
			} else {
				log.Println("shopSections is not in the expected format")
			}
		}
	}

	return contentPages, nil
}

func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}

func (f *Functions) GetRealIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	ip := r.RemoteAddr
	if colon := strings.LastIndex(ip, ":"); colon != -1 {
		return ip[:colon]
	}

	return ip
}

func (f *Functions) IsVPN(ip string) (bool, error) {
	return false, nil
}

type VersionInfo struct {
	Season int
	Build  float64
	CL     string
	Lobby  string
}

func (f *Functions) GetVersionInfo(req *http.Request) VersionInfo {
	memory := VersionInfo{
		Season: 0,
		Build:  0.0,
		CL:     "",
		Lobby:  "",
	}

	ua := req.Header.Get("User-Agent")
	if ua != "" {
		var CL string

		parts := strings.Split(ua, "-")
		if len(parts) > 3 {
			subParts := strings.Split(parts[3], ",")
			if len(subParts) > 0 {
				if _, err := strconv.Atoi(subParts[0]); err == nil {
					CL = subParts[0]
				} else {
					subParts = strings.Split(parts[3], " ")
					if len(subParts) > 0 {
						if _, err := strconv.Atoi(subParts[0]); err == nil {
							CL = subParts[0]
						}
					}
				}
			}
		}

		if CL == "" && len(parts) > 1 {
			subParts := strings.Split(parts[1], "+")
			if len(subParts) > 0 {
				if _, err := strconv.Atoi(subParts[0]); err == nil {
					CL = subParts[0]
				}
			}
		}

		defer func() {
			if r := recover(); r != nil {
				memory.Season = 2
				memory.Build = 2.0
				memory.CL = CL
				memory.Lobby = "LobbyWinterDecor"
			}
		}()

		if strings.Contains(ua, "Release-") {
			buildPart := strings.Split(strings.Split(ua, "Release-")[1], "-")[0]
			if strings.Count(buildPart, ".") == 2 {
				val := strings.Split(buildPart, ".")
				buildPart = val[0] + "." + val[1] + val[2]
			}
			seasonStr := strings.Split(buildPart, ".")[0]
			season, err1 := strconv.Atoi(seasonStr)
			build, err2 := strconv.ParseFloat(buildPart, 64)

			if err1 != nil || err2 != nil {
				panic("invalid season or build")
			}

			memory.Season = season
			memory.Build = build
			memory.CL = CL
			memory.Lobby = fmt.Sprintf("LobbySeason%d", season)
		} else {
			panic("no Release-")
		}
	}

	return memory
}
