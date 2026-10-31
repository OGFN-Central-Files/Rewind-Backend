package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"go-backend/src/database/models"
	"go-backend/src/global"
	"go-backend/src/services"
	"io"
	"net/http"
	"os"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
)

type Settings struct {
	BPublic        bool `json:"bPublic"`
	LauncherOnline bool `json:"launcherOnline"`
}

func InitDashboard() {
	http.HandleFunc("/api/v2/admin/dashboard", Dash)
	http.HandleFunc("/api/v2/admin/maindash", MainDash)
	http.HandleFunc("/api/v2/admin/verify", requireDashboardToken(AcceptLogin))
	http.HandleFunc("/api/v2/admin/log", requireDashboardToken(GetAuditLog))
	http.HandleFunc("/api/v2/admin/settings", requireDashboardToken(SettingsRequest))
	http.HandleFunc("/api/v2/admin/user", requireDashboardToken(UserQuery))
	http.HandleFunc("/api/v2/admin/ban", requireDashboardToken(BanUser))
	http.HandleFunc("/api/v2/admin/unban", requireDashboardToken(UnbanUser))
}

func requireDashboardToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Admin-Key") != "AdminDash123" {
			funcs.Log(services.WARNING, "LogDashboard", "Unauthorized Request!")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if r.URL.String() != "/api/v2/admin/verify" {
			cookie, err := r.Cookie("admin_key")
			if err != nil || cookie.Value != "AdminDash123" {
				funcs.Log(services.WARNING, "LogDashboard", "Invalid cookie accessed maindash!")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		next(w, r)
	}
}

func UnbanUser(w http.ResponseWriter, r *http.Request) {
	accountId := r.Header.Get("accountId")
	if accountId == "" {
		http.Error(w, `{"error": "accountId header is required"}`, http.StatusBadRequest)
		return
	}

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var existingAccount models.Account
	err2 := accountsCollection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&existingAccount)
	if err2 == nil {
		existingAccount.Banned = false
		existingAccount.Additional["banReason"] = "Unbanned by an admin."

		_, errv2 := accountsCollection.UpdateOne(
			context.Background(),
			bson.M{"accountid": accountId},
			bson.M{"$set": bson.M{
				"banned":     false,
				"additional": existingAccount.Additional,
			}},
		)
		if errv2 != nil {
			sendWebhook("Failed Manual-Unban " + existingAccount.Username)
			funcs.CreateErrorV2(
				"errors.com.epicgames.common.bad_request",
				"Failed to asign, a issue has been stated on the server.",
				[]string{}, -1, "", 401, w,
			)
			return
		}

		sendWebhook(existingAccount.Username + " Was Unbanned by an admin.")
		funcs.Log(services.WARNING, "LogDashboard", existingAccount.Username+" Was Unbanned by an admin.")
		w.WriteHeader(http.StatusOK)
		return
	}

	funcs.Log(services.ERROR, "LogDashboard", "Unban for "+accountId+" Failed, Error: "+err2.Error())
	w.WriteHeader(http.StatusInternalServerError)
}

func BanUser(w http.ResponseWriter, r *http.Request) {
	accountId := r.Header.Get("accountId")
	if accountId == "" {
		http.Error(w, `{"error": "accountId header is required"}`, http.StatusBadRequest)
		return
	}

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var existingAccount models.Account
	err2 := accountsCollection.FindOne(context.Background(), bson.M{"accountid": accountId}).Decode(&existingAccount)
	if err2 == nil {
		existingAccount.Banned = true
		existingAccount.Additional["banReason"] = "banned by an admin."

		_, errv2 := accountsCollection.UpdateOne(
			context.Background(),
			bson.M{"accountid": accountId},
			bson.M{"$set": bson.M{
				"banned":     true,
				"additional": existingAccount.Additional,
			}},
		)
		if errv2 != nil {
			sendWebhook("Failed manual-ban " + existingAccount.Username)
			funcs.CreateErrorV2(
				"errors.com.epicgames.common.bad_request",
				"Failed to asign, a issue has been stated on the server.",
				[]string{}, -1, "", 401, w,
			)
			return
		}

		sendWebhook(existingAccount.Username + " Was banned by an admin.")
		funcs.Log(services.WARNING, "LogDashboard", existingAccount.Username+" Was banned by an admin.")
		w.WriteHeader(http.StatusOK)
		return
	}

	funcs.Log(services.ERROR, "LogDashboard", "Ban for "+accountId+" Failed, Error: "+err2.Error())
	w.WriteHeader(http.StatusInternalServerError)
}

func UserQuery(w http.ResponseWriter, r *http.Request) {
	userSearch := r.Header.Get("User-Search")
	if userSearch == "" {
		http.Error(w, `{"error": "User-Search header is required"}`, http.StatusBadRequest)
		return
	}

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var users []models.Account

	cursor, err := accountsCollection.Find(context.Background(), bson.M{
		"username": bson.M{
			"$regex":   userSearch,
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

	if len(users) == 0 {
		var account models.Account
		err := accountsCollection.FindOne(context.Background(), bson.M{"accountid": userSearch}).Decode(&account)
		if err == nil {
			users = append(users, account)
		}
	}

	if len(users) == 0 {
		http.Error(w, `{"error": "No users found"}`, http.StatusNotFound)
		return
	}

	var response []map[string]interface{}
	for _, user := range users {
		banReason, banReasonExists := user.Additional["banReason"]
		if !banReasonExists {
			banReason = "Not banned"
		}

		response = append(response, map[string]interface{}{
			"id":        user.AccountID,
			"username":  user.Username,
			"accountId": user.AccountID,
			"banned":    user.Banned,
			"banReason": banReason,
		})
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, `{"error": "Unable to encode response"}`, http.StatusInternalServerError)
	}
}

func SettingsRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		bTestingPreviligedOnly, ok := global.TestingSettings["bTestingPreviligedOnly"].(bool)
		if !ok {
			http.Error(w, `{"error": "Invalid type for bTestingPreviligedOnly"}`, http.StatusInternalServerError)
			return
		}

		bPublic := !bTestingPreviligedOnly
		response := map[string]interface{}{
			"bPublic":        bPublic,
			"launcherOnline": global.LauncherSettings["bIsLauncherOnline"],
		}

		err := json.NewEncoder(w).Encode(response)
		if err != nil {
			http.Error(w, `{"error": "Unable to encode settings"}`, http.StatusInternalServerError)
			return
		}

		return
	}

	if r.Method == http.MethodPatch {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Unable to read request body", http.StatusBadRequest)
			return
		}

		var newSettings Settings
		err = json.Unmarshal(body, &newSettings)
		if err != nil {
			http.Error(w, "Invalid JSON format", http.StatusBadRequest)
			return
		}

		global.TestingSettings["bTestingPreviligedOnly"] = !newSettings.BPublic
		global.LauncherSettings["bIsLauncherOnline"] = newSettings.LauncherOnline

		if global.TestingSettings["bTestingPreviligedOnly"] == true {
			funcs.Log(services.WARNING, "LogPublic", "Only testers can now login!")
		} else {
			funcs.Log(services.WARNING, "LogPublic", "EVERYONE can now login!")
		}

		if newSettings.LauncherOnline {
			funcs.Log(services.WARNING, "LogLauncher", "Launcher is now ONLINE!")
		} else {
			funcs.Log(services.WARNING, "LogLauncher", "Launcher is now OFFLINE!")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]string{"message": "Settings updated successfully"}
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			http.Error(w, "Unable to send confirmation", http.StatusInternalServerError)
			return
		}
		return
	}

	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
}

func GetAuditLog(w http.ResponseWriter, r *http.Request) {
	htmlBytes, err := os.ReadFile("./app.log")
	if err != nil {
		http.Error(w, "Failed to load HTML file", http.StatusInternalServerError)
		return
	}
	htmlContent := string(htmlBytes)
	w.Header().Set("Content-Type", "text/text")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, htmlContent)
}

func AcceptLogin(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func MainDash(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("admin_key")
	if err != nil || cookie.Value != "AdminDash123" {
		funcs.Log(services.WARNING, "LogDashboard", "Invalid cookie accessed maindash!")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	htmlBytes, err := os.ReadFile("./frontend/home/main.html")
	if err != nil {
		http.Error(w, "Failed to load HTML file", http.StatusInternalServerError)
		return
	}
	htmlContent := string(htmlBytes)
	updatedHTML := strings.Replace(htmlContent, "%adminkey%", cookie.Value, -1)

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, updatedHTML)
}

func Dash(w http.ResponseWriter, r *http.Request) {
	htmlBytes, err := os.ReadFile("./frontend/home/dev.html")
	if err != nil {
		http.Error(w, "Failed to load HTML file", http.StatusInternalServerError)
		return
	}
	htmlContent := string(htmlBytes)

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, htmlContent)
}
