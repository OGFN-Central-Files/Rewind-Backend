package routes

import (
	"context"
	"encoding/json"
	"go-backend/src/database/models"
	XMPP "go-backend/src/definitions/xmpp"
	"go-backend/src/global"
	"go-backend/src/services"
	"log"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func getList(additional map[string]interface{}, key string) []map[string]interface{} {
	raw, ok := additional[key]
	if !ok {
		return nil
	}

	switch v := raw.(type) {
	case primitive.A:
		var result []map[string]interface{}
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				result = append(result, m)
			}
		}
		return result

	case []interface{}:
		var result []map[string]interface{}
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				result = append(result, m)
			}
		}
		return result

	case []map[string]interface{}:
		return v

	default:
		return nil
	}
}

func InitFriends() {
	http.HandleFunc("/api/v1/search/{accountId}", HandleSearch)
	http.HandleFunc("/friends/api/v1/{localAccountId}/friends/{TargetAccountId}", HandleFriendRequest)
	http.HandleFunc("/friends/api/v1/{accountId}/summary", HandleSummary)
}

func HandleSummary(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	accountId := parts[4]
	log.Println("Summary request for accountId:", accountId)

	response := map[string]interface{}{
		"friends":   []map[string]interface{}{},
		"incoming":  []map[string]interface{}{},
		"outgoing":  []map[string]interface{}{},
		"suggested": []interface{}{},
		"blocklist": []map[string]interface{}{},
		"settings": map[string]interface{}{
			"acceptInvites": "public",
		},
	}

	collection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var account models.Account
	err := collection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&account)
	if err != nil {
		log.Println("Error fetching account:", err)
		json.NewEncoder(w).Encode(response)
		return
	}
	if account.Additional == nil {
		log.Println("No Additional data found for account")
		json.NewEncoder(w).Encode(response)
		return
	}

	for _, f := range getList(account.Additional, "friends") {
		created := f["created"]
		var createdStr string

		if ts, ok := created.(float64); ok {
			createdStr = time.Unix(int64(ts), 0).UTC().Format(time.RFC3339)
		} else {
			createdStr = time.Now().AddDate(0, 0, -3).UTC().Format(time.RFC3339)
		}

		response["friends"] = append(response["friends"].([]map[string]interface{}), map[string]interface{}{
			"accountId": f["accountId"],
			"groups":    []interface{}{},
			"mutual":    0,
			"alias":     coalesceString(f["alias"]),
			"note":      "",
			"favorite":  false,
			"created":   createdStr,
		})
	}

	for _, i := range getList(account.Additional, "incoming") {
		response["incoming"] = append(response["incoming"].([]map[string]interface{}), map[string]interface{}{
			"accountId": i["accountId"],
			"mutual":    0,
			"favorite":  false,
			"created":   i["created"],
		})
	}

	for _, o := range getList(account.Additional, "outgoing") {
		response["outgoing"] = append(response["outgoing"].([]map[string]interface{}), map[string]interface{}{
			"accountId": o["accountId"],
			"favorite":  false,
		})
	}

	for _, b := range getList(account.Additional, "blocked") {
		response["blocklist"] = append(response["blocklist"].([]map[string]interface{}), map[string]interface{}{
			"accountId": b["accountId"],
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func coalesceString(value interface{}) string {
	if str, ok := value.(string); ok {
		return str
	}
	return ""
}

func HandleFriendRequest(w http.ResponseWriter, r *http.Request) {
	tokenManager := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenManager.VerifyToken(w, r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 7 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	localAccountId := parts[4]
	targetAccountId := parts[6]

	if localAccountId == targetAccountId {
		http.Error(w, "Cannot friend yourself", http.StatusBadRequest)
		return
	}

	collection := global.MongoDBClient.Database("fortnite").Collection("accounts")

	var from, to models.Account
	if err := collection.FindOne(context.Background(), bson.M{"accountid": localAccountId}).Decode(&from); err != nil {
		http.Error(w, "Sender not found", http.StatusForbidden)
		return
	}
	if err := collection.FindOne(context.Background(), bson.M{"accountid": targetAccountId}).Decode(&to); err != nil {
		http.Error(w, "Receiver not found", http.StatusForbidden)
		return
	}

	if from.Additional == nil {
		from.Additional = make(map[string]interface{})
	}
	if to.Additional == nil {
		to.Additional = make(map[string]interface{})
	}

	xmpp := XMPP.XmppService{}
	now := time.Now().UTC().Format(time.RFC3339)

	if r.Method == http.MethodDelete {
		removeByID := func(list []map[string]interface{}, id string) []map[string]interface{} {
			var newList []map[string]interface{}
			for _, item := range list {
				if item["accountId"] != id {
					newList = append(newList, item)
				}
			}
			return newList
		}

		from.Additional["friends"] = removeByID(getList(from.Additional, "friends"), targetAccountId)
		to.Additional["friends"] = removeByID(getList(to.Additional, "friends"), localAccountId)

		from.Additional["incoming"] = removeByID(getList(from.Additional, "incoming"), targetAccountId)
		from.Additional["outgoing"] = removeByID(getList(from.Additional, "outgoing"), targetAccountId)
		to.Additional["incoming"] = removeByID(getList(to.Additional, "incoming"), localAccountId)
		to.Additional["outgoing"] = removeByID(getList(to.Additional, "outgoing"), localAccountId)

		collection.UpdateOne(context.Background(), bson.M{"accountid": localAccountId}, bson.M{"$set": bson.M{"additional": from.Additional}})
		collection.UpdateOne(context.Background(), bson.M{"accountid": targetAccountId}, bson.M{"$set": bson.M{"additional": to.Additional}})

		xmpp.SendXmppMessageToClient(from.AccountID, map[string]interface{}{
			"type":      "com.epicgames.friends.core.apiobjects.FriendRemoval",
			"timestamp": now,
			"payload": map[string]interface{}{
				"accountId": to.AccountID,
			},
		})
		xmpp.SendXmppMessageToClient(to.AccountID, map[string]interface{}{
			"type":      "com.epicgames.friends.core.apiobjects.FriendRemoval",
			"timestamp": now,
			"payload": map[string]interface{}{
				"accountId": from.AccountID,
			},
		})

		w.WriteHeader(http.StatusNoContent)
		return
	}

	incoming := getList(from.Additional, "incoming")
	outgoing := getList(from.Additional, "outgoing")

	foundInIncoming := false
	for _, i := range incoming {
		if i["accountId"] == targetAccountId {
			foundInIncoming = true
			break
		}
	}

	if foundInIncoming {
		from.Additional["friends"] = append(getList(from.Additional, "friends"), map[string]interface{}{
			"accountId": targetAccountId,
			"alias":     "",
			"created":   now,
			"mutual":    0,
			"favorite":  false,
		})
		to.Additional["friends"] = append(getList(to.Additional, "friends"), map[string]interface{}{
			"accountId": localAccountId,
			"alias":     "",
			"created":   now,
			"mutual":    0,
			"favorite":  false,
		})
		from.Additional["incoming"] = removeAccountByID(incoming, targetAccountId)
		to.Additional["outgoing"] = removeAccountByID(getList(to.Additional, "outgoing"), localAccountId)

		xmpp.SendXmppMessageToClient(from.AccountID, map[string]interface{}{
			"type":      "com.epicgames.friends.core.apiobjects.Friend",
			"timestamp": now,
			"payload": map[string]interface{}{
				"accountId": to.AccountID,
				"status":    "ACCEPTED",
				"direction": "OUTBOUND",
				"created":   now,
				"favorite":  false,
			},
		})
		xmpp.SendXmppMessageToClient(to.AccountID, map[string]interface{}{
			"type":      "com.epicgames.friends.core.apiobjects.Friend",
			"timestamp": now,
			"payload": map[string]interface{}{
				"accountId": from.AccountID,
				"status":    "ACCEPTED",
				"direction": "INBOUND",
				"created":   now,
				"favorite":  false,
			},
		})
	} else {
		alreadySent := false
		for _, o := range outgoing {
			if o["accountId"] == targetAccountId {
				alreadySent = true
				break
			}
		}
		if !alreadySent {
			from.Additional["outgoing"] = append(outgoing, map[string]interface{}{
				"accountId": targetAccountId,
				"created":   now,
				"favorite":  false,
			})
			to.Additional["incoming"] = append(getList(to.Additional, "incoming"), map[string]interface{}{
				"accountId": localAccountId,
				"created":   now,
				"mutual":    0,
				"favorite":  false,
			})

			xmpp.SendXmppMessageToClient(from.AccountID, map[string]interface{}{
				"type":      "com.epicgames.friends.core.apiobjects.Friend",
				"timestamp": now,
				"payload": map[string]interface{}{
					"accountId": to.AccountID,
					"status":    "PENDING",
					"direction": "OUTBOUND",
					"created":   now,
					"favorite":  false,
				},
			})
			xmpp.SendXmppMessageToClient(to.AccountID, map[string]interface{}{
				"type":      "com.epicgames.friends.core.apiobjects.Friend",
				"timestamp": now,
				"payload": map[string]interface{}{
					"accountId": from.AccountID,
					"status":    "PENDING",
					"direction": "INBOUND",
					"created":   now,
					"favorite":  false,
				},
			})
		}
	}

	collection.UpdateOne(context.Background(), bson.M{"accountid": localAccountId}, bson.M{"$set": bson.M{"additional": from.Additional}})
	collection.UpdateOne(context.Background(), bson.M{"accountid": targetAccountId}, bson.M{"$set": bson.M{"additional": to.Additional}})

	w.WriteHeader(http.StatusNoContent)
}

func removeAccountByID(list []map[string]interface{}, id string) []map[string]interface{} {
	var filtered []map[string]interface{}
	for _, item := range list {
		if itemId, ok := item["accountId"].(string); ok && itemId != id {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func HandleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	prefix := query.Get("prefix")

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var users []models.Account
	cursor, err := accountsCollection.Find(context.Background(), bson.M{
		"username": bson.M{
			"$regex":   "^" + prefix,
			"$options": "i",
		},
	})
	if err != nil {
		http.Error(w, `{"error": "Unable to query the database"}`, http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	for cursor.Next(context.Background()) {
		var account models.Account
		if err := cursor.Decode(&account); err != nil {
			http.Error(w, `{"error": "Unable to decode account"}`, http.StatusInternalServerError)
			return
		}
		users = append(users, account)
	}

	response := []map[string]interface{}{}

	for _, user := range users {
		if len(response) >= 100 {
			break
		}

		matchType := "prefix"
		if strings.ToLower(prefix) == strings.ToLower(user.Username) {
			matchType = "exact"
		}

		response = append(response, map[string]interface{}{
			"accountId": user.AccountID,
			"matches": []map[string]interface{}{
				{
					"value":    user.Username,
					"platform": "epic",
				},
			},
			"matchType":    matchType,
			"epicMutuals":  0,
			"sortPosition": len(response),
		})
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, `{"error": "Unable to encode response"}`, http.StatusInternalServerError)
	}
}
