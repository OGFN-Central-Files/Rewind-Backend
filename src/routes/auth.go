package routes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"go-backend/src/database/models"
	"go-backend/src/global"
	"go-backend/src/services"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"go.mongodb.org/mongo-driver/bson"
)

type RequestBody struct {
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
}

func getRealIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return strings.Split(ip, ",")[0]
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

func InitAuth() {
	http.HandleFunc("/account/api/oauth/token", HandleOauthTokenRequest)
	http.HandleFunc("/account/api/public/account/{accountId}", HandlePublicAccountV1)
	http.HandleFunc("/account/api/public/account", HandlePublicAccountV2)
	http.HandleFunc("/account/api/public/account/{accountId}/externalAuths", HandleExternalAuths)
	http.HandleFunc("/datarouter/api/v1/public/data", HandleDataRouter)
	http.HandleFunc("/fortnite/api/v2/versioncheck/{platform}", HandleVersionCheck)
	http.HandleFunc("/fortnite/api/game/v2/tryPlayOnPlatform/account/{accountId}", TryPlayOnPlatform)
	http.HandleFunc("/account/api/oauth/verify", oAuthVerify)
	http.HandleFunc("/auth/v1/oauth/token", handleTokenRequestV1)

	http.HandleFunc("/epic/oauth/v2/token", proxyRequestAether)
	http.HandleFunc("/epic/id/v2/sdk/accounts", proxyRequestAether)
	http.HandleFunc("/v1/epic-settings/public/users/{accountId}/values", proxyRequestAether)

	http.HandleFunc("/account/api/oauth/sessions/kill/*", KillSession)
	http.HandleFunc("/account/api/oauth/sessions/kill", KillSession)
}

func KillSession(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func handleSdkAccountsInfo(w http.ResponseWriter, r *http.Request) {
	ip := getRealIP(r)

	global.IPToUserMutex.Lock()
	userInfo, _ := global.IPToUserMap[ip]
	global.IPToUserMutex.Unlock()

	response := map[string]interface{}{
		"account_id":        userInfo.AccountID,
		"displayName":       userInfo.AccountID,
		"preferredLanguage": "en",
		"cabinedMode":       false,
		"empty":             false,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleEpicOauthTokenV2(w http.ResponseWriter, r *http.Request) {

	ip := getRealIP(r)

	global.IPToUserMutex.Lock()
	userInfo, _ := global.IPToUserMap[ip]
	global.IPToUserMutex.Unlock()

	tokenStr := strings.TrimPrefix(userInfo.AccessToken, "eg1~")
	decodedAccess, _ := jwt.Parse(tokenStr, nil)
	claims := decodedAccess.Claims.(jwt.MapClaims)

	if claims == nil || claims["sub"] == nil {
		funcs.CreateError(
			"errors.com.epicgames.oauth.refresh_token",
			"refresh token is required",
			[]string{},
			400,
			"missing_refresh_token",
			"prod",
			"refresh token is required",
			"missing_refresh_token",
			w,
		)
		return
	}

	creationDate, err := time.Parse(time.RFC3339, claims["creation_date"].(string))
	if err != nil {
		http.Error(w, "Failed to parse creation date", http.StatusInternalServerError)
		return
	}

	decodedRefresh, err := jwt.Parse(strings.TrimPrefix(userInfo.RefreshToken, "eg1~"), func(token *jwt.Token) (interface{}, error) {
		return []byte("JWT_TOKEN_1231289SNN"), nil
	})
	if err != nil || decodedRefresh == nil {
		http.Error(w, "Failed to parse refresh token", http.StatusInternalServerError)
		return
	}

	refreshClaims, ok := decodedRefresh.Claims.(jwt.MapClaims)
	if !ok {
		http.Error(w, "Invalid refresh token claims", http.StatusInternalServerError)
		return
	}

	creationDateRefresh, ok := refreshClaims["creation_date"].(string)
	if !ok {
		http.Error(w, "Invalid creation_date in refresh token", http.StatusInternalServerError)
		return
	}

	parsedCreationDateRefresh, err := time.Parse(time.RFC3339, creationDateRefresh)
	if err != nil {
		http.Error(w, "Failed to parse creation_date in refresh token", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"scope":               "basic_profile friends_list openid presence",
		"token_type":          "bearer",
		"access_token":        userInfo.AccessToken,
		"expires_in":          time.Until(funcs.DateAddHours(creationDate, 4)),
		"expires_at":          funcs.DateAddHours(creationDate, 4).Format(time.RFC3339),
		"refresh_token":       userInfo.RefreshToken,
		"refresh_expires_in":  int(time.Until(funcs.DateAddHours(parsedCreationDateRefresh, 8))),
		"refresh_expires_at":  funcs.DateAddHours(parsedCreationDateRefresh, 8).Format(time.RFC3339),
		"account_id":          userInfo.AccountID,
		"client_id":           userInfo.ClientID,
		"application_id":      userInfo.AccessToken,
		"selected_account_id": userInfo.AccountID,
		"id_token":            userInfo.AccessToken,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleTokenRequestV1(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"access_token":    "aventis",
		"token_type":      "bearer",
		"expires_at":      "9999-12-31T23:59:59.999Z",
		"features":        []string{"AntiCheat", "Connect", "Ecom"},
		"organization_id": "aventis",
		"product_id":      "prod-fn",
		"sandbox_id":      "fn",
		"deployment_id":   "aventis",
		"expires_in":      3599,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func oAuthVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(`{"error": "Method Not Allowed", "message": "Only GET requests are allowed"}`))
		return
	}

	funcs := &services.Functions{}
	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")

	user := tokenCreation.VerifyToken(w, r)
	if user == nil {
		return
	}

	token := r.Header.Get("Authorization")

	tokenStr := strings.TrimPrefix(token, "bearer eg1~")
	decodedAccess, _ := jwt.Parse(tokenStr, nil)
	claims := decodedAccess.Claims.(jwt.MapClaims)

	if claims == nil || claims["sub"] == nil {
		funcs.CreateError(
			"errors.com.epicgames.oauth.refresh_token",
			"refresh token is required",
			[]string{},
			400,
			"missing_refresh_token",
			"prod",
			"refresh token is required",
			"missing_refresh_token",
			w,
		)
		return
	}

	creationDate, err := time.Parse(time.RFC3339, claims["creation_date"].(string))
	if err != nil {
		http.Error(w, "Failed to parse creation date", http.StatusInternalServerError)
		return
	}

	//funcs.Log(services.INFO, "LogAuth", "User' oAuth Verification request accepted! AccountID:"+user.AccountID)
	response := map[string]interface{}{
		"token":           tokenStr,
		"session_id":      claims["jti"],
		"token_type":      "bearer",
		"client_id":       claims["clid"],
		"internal_client": true,
		"client_service":  "fortnite",
		"account_id":      user.AccountID,
		"expires_in":      time.Until(funcs.DateAddHours(creationDate, 4)),
		"expires_at":      funcs.DateAddHours(creationDate, 4).Format(time.RFC3339),
		"auth_method":     claims["am"],
		"display_name":    user.Username,
		"app":             "fortnite",
		"in_app_id":       user.AccountID,
		"device_id":       claims["dvid"],
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func TryPlayOnPlatform(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(`{"error": "Method Not Allowed", "message": "Only POST requests are allowed"}`))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("true"))
}

func HandleVersionCheck(w http.ResponseWriter, r *http.Request) {
	// funcs := &services.Functions{}

	response := map[string]interface{}{
		"type": "NO_UPDATE",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	// platform := mux.Vars(r)["platform"]
	// version := r.URL.Query().Get("version")
	// if platform != "windows" {
	// 	funcs.CreateError(
	// 		"errors.com.epicgames.common.invald_platform",
	// 		"This platform is currently unsupportive for fortnite.",
	// 		[]string{},
	// 		404,
	// 		"any",
	// 		"prod",
	// 		"This platform is currently unsupportive for fortnite.",
	// 		"invald_platform",
	// 		w,
	// 	)

	// 	return
	// }
	// if version != global.CurrentVersionInfo.FortniteRelease {
	// 	response := map[string]interface{}{
	// 		"type": "UPDATE",
	// 	}

	// 	w.Header().Set("Content-Type", "application/json")
	// 	json.NewEncoder(w).Encode(response)
	// } else {
	// 	response := map[string]interface{}{
	// 		"type": "NO_UPDATE",
	// 	}

	// 	w.Header().Set("Content-Type", "application/json")
	// 	json.NewEncoder(w).Encode(response)
	// }
}

func HandleDataRouter(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func HandleExternalAuths(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func HandlePublicAccountV2(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Query().Get("accountId") == "" {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(`{"error": "Method Not Allowed", "message": "Only GET requests are allowed"}`))
		return
	}

	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenCreation.VerifyToken(w, r)
	if user == nil {
		return
	}

	accountIds := r.URL.Query()["accountId"]
	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	response := []map[string]interface{}{}

	if len(accountIds) == 1 {
		accountId := accountIds[0]
		var existingAccount models.Account
		err := accountsCollection.FindOne(context.Background(), bson.M{"accountid": accountId, "banned": false}).Decode(&existingAccount)
		if err == nil {
			response = append(response, map[string]interface{}{
				"id":            existingAccount.AccountID,
				"displayName":   existingAccount.Username,
				"externalAuths": map[string]interface{}{},
			})
		}
	} else {
		filter := bson.M{"accountid": bson.M{"$in": accountIds}, "banned": false}
		cursor, err := accountsCollection.Find(context.Background(), filter)
		if err == nil {
			defer cursor.Close(context.Background())
			for cursor.Next(context.Background()) {
				var user models.Account
				if err := cursor.Decode(&user); err == nil {
					if len(response) >= 100 {
						break
					}
					response = append(response, map[string]interface{}{
						"id":            user.AccountID,
						"displayName":   user.Username,
						"externalAuths": map[string]interface{}{},
					})
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func HandlePublicAccountV1(w http.ResponseWriter, r *http.Request) {
	funcs := &services.Functions{}
	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(`{"error": "Method Not Allowed", "message": "Only GET requests are allowed"}`))
		funcs.Log(services.WARNING, "BYPASS", "A user tried accessing a locked endpoint or a Method changed!")
		return
	}

	user := tokenCreation.VerifyToken(w, r)
	if user == nil {
		return
	}

	emailParts := strings.Split(user.Email, "@")
	hiddenEmail := fmt.Sprintf("[hidden]@%s", emailParts[1])

	response := map[string]interface{}{
		"id":                         user.AccountID,
		"displayName":                user.Username,
		"name":                       "Rewind",
		"email":                      hiddenEmail,
		"failedLoginAttempts":        0,
		"lastLogin":                  time.Now().Format(time.RFC3339),
		"numberOfDisplayNameChanges": 0,
		"ageGroup":                   "UNKNOWN",
		"headless":                   false,
		"country":                    "US",
		"lastName":                   "Server",
		"preferredLanguage":          "en",
		"canUpdateDisplayName":       false,
		"tfaEnabled":                 false,
		"emailVerified":              true,
		"minorVerified":              false,
		"minorExpected":              false,
		"minorStatus":                "NOT_MINOR",
		"cabinedMode":                false,
		"hasHashedEmail":             false,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func HandleOauthTokenRequest(w http.ResponseWriter, r *http.Request) {
	funcs := &services.Functions{}
	if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
		funcs.CreateError(
			"errors.com.epicgames.oauth.unsupported_grant_type",
			"unsupported_grant_type",
			[]string{},
			1013,
			"unsupported_grant_type",
			"prod",
			"unsupported_grant_type",
			"unsupported_grant_type",
			w,
		)
		return
	}

	if err := r.ParseForm(); err != nil {
		funcs.CreateError(
			"errors.com.epicgames.oauth.unsupported_grant_type",
			"unsupported_grant_type",
			[]string{},
			1013,
			"unsupported_grant_type",
			"prod",
			"unsupported_grant_type",
			"unsupported_grant_type",
			w,
		)
		return
	}

	grantType := r.FormValue("grant_type")
	username := r.FormValue("username")
	password := r.FormValue("password")
	refreshToken := r.FormValue("refresh_token")

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		funcs.CreateError(
			"errors.com.epicgames.oauth.unsupported_grant_type",
			"unsupported_grant_type",
			[]string{},
			1013,
			"unsupported_grant_type",
			"prod",
			"unsupported_grant_type",
			"unsupported_grant_type",
			w,
		)
		return
	}

	clientId, err := extractClientId(authHeader)
	if err != nil {
		funcs.CreateError(
			"errors.com.epicgames.oauth.unsupported_grant_type",
			"unsupported_grant_type",
			[]string{},
			1013,
			"unsupported_grant_type",
			"prod",
			"unsupported_grant_type",
			"unsupported_grant_type",
			w,
		)
		return
	}

	switch grantType {
	case "client_credentials":
		handleClientCredentials(w, r, clientId)
	case "password":
		handlePasswordGrant(w, clientId, username, password, r)
	case "refresh_token":
		handleRefreshToken(w, clientId, refreshToken)
	default:
		funcs.CreateError(
			"errors.com.epicgames.oauth.unsupported_grant_type",
			"unsupported_grant_type",
			[]string{},
			1013,
			"unsupported_grant_type",
			"prod",
			"unsupported_grant_type",
			"unsupported_grant_type",
			w,
		)
	}
}

func extractClientId(authHeader string) (string, error) {
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Basic") {
		return "", errors.New("invalid Authorization header format: must be 'Basic <base64>'")
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", errors.New("failed to decode Authorization header: invalid base64")
	}

	clientCredentials := strings.Split(string(decoded), ":")
	if len(clientCredentials) != 2 {
		return "", errors.New("invalid client credentials format: must be 'clientId:clientSecret'")
	}

	clientId := clientCredentials[0]
	if clientId == "" {
		return "", errors.New("invalid client ID: clientId cannot be empty")
	}

	return clientId, nil
}
func handleClientCredentials(w http.ResponseWriter, r *http.Request, clientId string) {
	funcs := &services.Functions{}
	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")

	ip := strings.Split(r.RemoteAddr, ":")[0]
	token, err := tokenCreation.CreateClient(clientId, "client_credentials", ip, 4)
	if err != nil {
		http.Error(w, "Failed to create client token", http.StatusInternalServerError)
		return
	}

	decodedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte("JWT_TOKEN_1231289SNN"), nil
	})
	if err != nil {
		http.Error(w, "Failed to parse token", http.StatusInternalServerError)
		return
	}

	claims := decodedToken.Claims.(jwt.MapClaims)
	creationDate, err := time.Parse(time.RFC3339, claims["creation_date"].(string))
	if err != nil {
		http.Error(w, "Failed to parse creation date", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"access_token":    "eg1~" + token,
		"expires_in":      int(time.Until(funcs.DateAddHours(creationDate, 4))),
		"expires_at":      funcs.DateAddHours(creationDate, 4).Format(time.RFC3339),
		"token_type":      "bearer",
		"client_id":       clientId,
		"internal_client": true,
		"client_service":  "fortnite",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handlePasswordGrant(w http.ResponseWriter, clientId, username, password string, r *http.Request) {
	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var existingAccount *models.Account
	err := accountsCollection.FindOne(context.Background(), bson.M{"email": username}).Decode(&existingAccount)

	funcs := &services.Functions{}
	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")

	if err != nil || existingAccount.Password != password {
		funcs.CreateErrorV2(
			"errors.com.epicgames.account.invalid_account_credentials",
			"Your e-mail and/or password are incorrect. Please check them and try again.",
			[]string{}, 18031, "invalid_grant", 400, w,
		)
		return
	}

	if existingAccount.Banned {
		funcs.CreateErrorV2(
			"errors.com.epicgames.account.account_not_active",
			"You have been permanently banned from Fortnite.",
			[]string{}, -1, "", 400, w,
		)
		return
	}

	global.BerylliumClients[existingAccount.AccountID] = map[string]interface{}{ // resets AC connection
		"bAuthenticated": false,
	}

	deviceId := funcs.MakeID()
	accessToken, err_access := tokenCreation.CreateAccess(existingAccount, clientId, "password", deviceId, 8)
	if err_access != nil {
		http.Error(w, "Failed to create client token", http.StatusInternalServerError)
		return
	}

	refreshToken, err_refresh := tokenCreation.CreateRefresh(existingAccount, clientId, "password", deviceId, 8)
	if err_refresh != nil {
		http.Error(w, "Failed to create client token", http.StatusInternalServerError)
		return
	}

	global.RefreshTokens[refreshToken] = refreshToken
	global.AccesssTokens[accessToken] = accessToken

	decodedAccess, err := jwt.Parse(accessToken, func(token *jwt.Token) (interface{}, error) {
		return []byte("JWT_TOKEN_1231289SNN"), nil
	})
	if err != nil || decodedAccess == nil {
		http.Error(w, "Failed to parse access token", http.StatusInternalServerError)
		return
	}

	decodedRefresh, err := jwt.Parse(refreshToken, func(token *jwt.Token) (interface{}, error) {
		return []byte("JWT_TOKEN_1231289SNN"), nil
	})
	if err != nil || decodedRefresh == nil {
		http.Error(w, "Failed to parse refresh token", http.StatusInternalServerError)
		return
	}

	accessClaims, ok := decodedAccess.Claims.(jwt.MapClaims)
	if !ok {
		http.Error(w, "Invalid access token claims", http.StatusInternalServerError)
		return
	}

	refreshClaims, ok := decodedRefresh.Claims.(jwt.MapClaims)
	if !ok {
		http.Error(w, "Invalid refresh token claims", http.StatusInternalServerError)
		return
	}

	creationDateAccess, ok := accessClaims["creation_date"].(string)
	if !ok {
		http.Error(w, "Invalid creation_date in access token", http.StatusInternalServerError)
		return
	}

	creationDateRefresh, ok := refreshClaims["creation_date"].(string)
	if !ok {
		http.Error(w, "Invalid creation_date in refresh token", http.StatusInternalServerError)
		return
	}

	parsedCreationDateAccess, err := time.Parse(time.RFC3339, creationDateAccess)
	if err != nil {
		http.Error(w, "Failed to parse creation_date in access token", http.StatusInternalServerError)
		return
	}

	parsedCreationDateRefresh, err := time.Parse(time.RFC3339, creationDateRefresh)
	if err != nil {
		http.Error(w, "Failed to parse creation_date in refresh token", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"access_token":       "eg1~" + accessToken,
		"expires_in":         int(time.Until(funcs.DateAddHours(parsedCreationDateAccess, 8))),
		"expires_at":         funcs.DateAddHours(parsedCreationDateAccess, 8).Format(time.RFC3339),
		"token_type":         "bearer",
		"refresh_token":      "eg1~" + refreshToken,
		"refresh_expires":    int(time.Until(funcs.DateAddHours(parsedCreationDateRefresh, 8))),
		"refresh_expires_at": funcs.DateAddHours(parsedCreationDateRefresh, 8).Format(time.RFC3339),
		"account_id":         existingAccount.AccountID,
		"client_id":          clientId,
		"internal_client":    true,
		"client_service":     "fortnite",
		"displayName":        existingAccount.Username,
		"app":                "fortnite",
		"in_app_id":          existingAccount.AccountID,
		"device_id":          deviceId,
	}

	ip := getRealIP(r)

	global.IPToUserMutex.Lock()
	global.IPToUserMap[ip] = global.IPUserInfo{
		AccountID:    existingAccount.AccountID,
		Username:     existingAccount.Username,
		AccessToken:  "eg1~" + accessToken,
		RefreshToken: "eg1~" + refreshToken,
		ClientID:     clientId,
		DeviceID:     deviceId,
	}
	global.IPToUserMutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleRefreshToken(w http.ResponseWriter, clientId, refreshToken string) {
	funcs := &services.Functions{}
	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")

	if refreshToken == "" {
		funcs.CreateError(
			"errors.com.epicgames.oauth.refresh_token",
			"refresh token is required",
			[]string{},
			400,
			"missing_refresh_token",
			"prod",
			"refresh token is required",
			"missing_refresh_token",
			w,
		)
		return
	}

	decodedRefresh, err := jwt.Parse(refreshToken, nil)
	if err != nil || funcs.DateAddHours(decodedRefresh.Claims.(jwt.MapClaims)["creation_date"].(time.Time), 8).Before(time.Now()) {
		funcs.CreateError(
			"errors.com.epicgames.oauth.refresh_token",
			"refresh token is required",
			[]string{},
			400,
			"missing_refresh_token",
			"prod",
			"refresh token is required",
			"missing_refresh_token",
			w,
		)
		return
	}

	accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
	var existingAccount *models.Account
	accountIdDecoded := decodedRefresh.Claims.(jwt.MapClaims)["sub"].(string)
	err2 := accountsCollection.FindOne(context.Background(), bson.M{"accountId": accountIdDecoded}).Decode(&existingAccount)

	if err2 != nil {
		funcs.CreateError(
			"errors.com.epicgames.oauth.refresh_token",
			"refresh token is required",
			[]string{},
			400,
			"missing_refresh_token",
			"prod",
			"refresh token is required",
			"missing_refresh_token",
			w,
		)
		return
	}

	deviceId := funcs.MakeID()
	accessToken, err_access := tokenCreation.CreateAccess(existingAccount, clientId, "refresh_token", deviceId, 8)
	if err_access != nil {
		funcs.CreateError(
			"errors.com.epicgames.oauth.refresh_token",
			"refresh token is required",
			[]string{},
			400,
			"missing_refresh_token",
			"prod",
			"refresh token is required",
			"missing_refresh_token",
			w,
		)
		return
	}

	newRefreshToken, err_refresh := tokenCreation.CreateRefresh(existingAccount, clientId, "refresh_token", deviceId, 8)
	if err_refresh != nil {
		funcs.CreateError(
			"errors.com.epicgames.oauth.refresh_token",
			"refresh token is required",
			[]string{},
			400,
			"missing_refresh_token",
			"prod",
			"refresh token is required",
			"missing_refresh_token",
			w,
		)
		return
	}

	global.RefreshTokens[refreshToken] = refreshToken
	global.AccesssTokens[accessToken] = accessToken

	decodedAccess, _ := jwt.Parse(accessToken, nil)

	response := map[string]interface{}{
		"access_token":       "eg1~" + accessToken,
		"expires_in":         int(time.Until(funcs.DateAddHours(decodedAccess.Claims.(jwt.MapClaims)["creation_date"].(time.Time), 8))),
		"expires_at":         funcs.DateAddHours(decodedAccess.Claims.(jwt.MapClaims)["creation_date"].(time.Time), 8).Format(time.RFC3339),
		"token_type":         "bearer",
		"refresh_token":      "eg1~" + newRefreshToken,
		"refresh_expires":    int(time.Until(funcs.DateAddHours(decodedRefresh.Claims.(jwt.MapClaims)["creation_date"].(time.Time), 8))),
		"refresh_expires_at": funcs.DateAddHours(decodedRefresh.Claims.(jwt.MapClaims)["creation_date"].(time.Time), 8).Format(time.RFC3339),
		"account_id":         existingAccount.AccountID,
		"client_id":          clientId,
		"internal_client":    true,
		"client_service":     "fortnite",
		"displayName":        existingAccount.Username,
		"app":                "fortnite",
		"in_app_id":          existingAccount.AccountID,
		"device_id":          deviceId,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
