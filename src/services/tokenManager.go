package services

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go-backend/src/database/models"
	"go-backend/src/global"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
)

type TokenManager struct {
	JWTSecret string
}

type VerifyTokenResponse struct {
	BVerified bool `json:"bVerified"`
}

func NewTokenManager(jwtSecret string) *TokenManager {
	return &TokenManager{
		JWTSecret: jwtSecret,
	}
}

func (tm *TokenManager) VerifyToken(w http.ResponseWriter, r *http.Request) *models.Account {
	funcs := &Functions{}
	authHeader := r.Header.Get("Authorization")

	if authHeader == "" || !strings.HasPrefix(authHeader, "bearer eg1~") {
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
		return nil
	}

	tokenStr := strings.TrimPrefix(authHeader, "bearer eg1~")

	if tokenStr == "" {
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
		return nil
	}

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
		return nil
	}

	tokenSessionId := claims["token_session_id"].(string)
	if global.SessionSettings["bAllowSessionAfterRestart"].(bool) == true && tokenSessionId != global.SessionSettings["CurrentSessionId"].(string) {
		funcs.CreateErrorV2(
			"errors.com.epicgames.common.authorization.authorization_failed",
			"Authorization failed",
			[]string{}, 1032, "", 401, w,
		)
		return nil
	}

	creationDateString := claims["creation_date"].(string)
	creationDate, err := time.Parse(time.RFC3339, creationDateString)
	if err != nil {
		fmt.Println("Error parsing creation date:", err)
		return nil
	}

	currentTime := time.Now()
	duration := currentTime.Sub(creationDate)

	if duration < 8*time.Hour {
		accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
		var existingAccount *models.Account
		accountIdDecoded := claims["sub"].(string)
		err2 := accountsCollection.FindOne(context.Background(), bson.M{"accountid": accountIdDecoded}).Decode(&existingAccount)

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
			return nil
		}

		if global.TestingSettings["bTestingPreviligedOnly"].(bool) == true && existingAccount.Password != global.TestingSettings["GuildBypassPasswords"].(string) {
			additionalData := existingAccount.Additional
			if value, ok := additionalData["bTestingPriviliged"].(bool); !ok || !value {
				funcs.CreateErrorV2(
					"errors.com.epicgames.account.account_not_active",
					"You have been logged out from Fortnite.",
					[]string{}, -1, "", 400, w,
				)
				return nil
			}
		}

		if existingAccount.Banned {
			global.GiftReceivedUsers[existingAccount.AccountID] = map[string]interface{}{
				"bAuthenticated": false,
			}

			funcs.CreateErrorV2(
				"errors.com.epicgames.account.account_not_active",
				"You have been permanently banned from Fortnite.",
				[]string{}, -1, "", 400, w,
			)
			return nil
		}

		return existingAccount
	} else {
		return nil
	}
}

type TokenRequest struct {
	Token string `json:"token"`
	HWID  string `json:"hwid"`
}

type DeauthRequest struct {
	Token string `json:"token"`
}

type BanRequest struct {
	Token  string `json:"token"`
	Reason string `json:"message"`
}

func (tm *TokenManager) VerifyTokenBeryllium(w http.ResponseWriter, r *http.Request, token string) *models.Account {
	funcs := &Functions{}
	authHeader := token

	if authHeader == "" || !strings.HasPrefix(authHeader, "eg1~") {
		funcs.CreateError(
			"errors.com.epicgames.oauth.refresh_token",
			"refresh token is required",
			[]string{},
			401,
			"missing_refresh_token",
			"prod",
			"refresh token is required",
			"missing_refresh_token",
			w,
		)
		return nil
	}

	tokenStr := strings.TrimPrefix(authHeader, "eg1~")

	if tokenStr == "" {
		funcs.CreateError(
			"errors.com.epicgames.oauth.refresh_token",
			"refresh token is required",
			[]string{},
			401,
			"missing_refresh_token",
			"prod",
			"refresh token is required",
			"missing_refresh_token",
			w,
		)
		return nil
	}

	decodedAccess, _ := jwt.Parse(tokenStr, nil)
	claims := decodedAccess.Claims.(jwt.MapClaims)

	if claims == nil || claims["sub"] == nil {
		funcs.CreateError(
			"errors.com.epicgames.oauth.refresh_token",
			"refresh token is required",
			[]string{},
			401,
			"missing_refresh_token",
			"prod",
			"refresh token is required",
			"missing_refresh_token",
			w,
		)
		return nil
	}

	tokenSessionId := claims["token_session_id"].(string)
	if global.SessionSettings["bAllowSessionAfterRestart"].(bool) == true && tokenSessionId != global.SessionSettings["CurrentSessionId"].(string) {
		funcs.CreateErrorV2(
			"errors.com.epicgames.common.authorization.authorization_failed",
			"Authorization failed",
			[]string{}, 1032, "", 401, w,
		)
		return nil
	}

	creationDateString := claims["creation_date"].(string)
	creationDate, err := time.Parse(time.RFC3339, creationDateString)
	if err != nil {
		fmt.Println("Error parsing creation date:", err)
		return nil
	}

	currentTime := time.Now()
	duration := currentTime.Sub(creationDate)

	if duration < 8*time.Hour {
		accountsCollection := global.MongoDBClient.Database("fortnite").Collection("accounts")
		var existingAccount *models.Account
		accountIdDecoded := claims["sub"].(string)
		err2 := accountsCollection.FindOne(context.Background(), bson.M{"accountid": accountIdDecoded}).Decode(&existingAccount)

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
			return nil
		}

		if global.TestingSettings["bTestingPreviligedOnly"].(bool) == true && existingAccount.Password != global.TestingSettings["GuildBypassPasswords"].(string) {
			additionalData := existingAccount.Additional
			if value, ok := additionalData["bTestingPriviliged"].(bool); !ok || !value {
				funcs.CreateErrorV2(
					"errors.com.epicgames.account.account_not_active",
					"You have been logged out from Fortnite.",
					[]string{}, -1, "", 400, w,
				)
				return nil
			}
		}

		if existingAccount.Banned {
			global.GiftReceivedUsers[existingAccount.AccountID] = map[string]interface{}{
				"bAuthenticated": false,
			}

			funcs.CreateErrorV2(
				"errors.com.epicgames.account.account_not_active",
				"You have been permanently banned from Fortnite.",
				[]string{}, -1, "", 400, w,
			)
			return nil
		}

		return existingAccount
	} else {
		return nil
	}
}

func (tm *TokenManager) CreateClient(clientId, grantType, ip string, hoursExpire int) (string, error) {
	claims := jwt.MapClaims{
		"p":             MakeID(),
		"clid":          clientId,
		"t":             "client",
		"am":            grantType,
		"ip":            ip,
		"creation_date": time.Now().UTC().Format(time.RFC3339),
		"hours_expire":  hoursExpire,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(tm.JWTSecret))
}

func (tm *TokenManager) CreateAccess(account *models.Account, clientId, grantType, deviceId string, hoursExpire int) (string, error) {
	claims := jwt.MapClaims{
		"p":                MakeID(),
		"clid":             clientId,
		"t":                "access",
		"am":               grantType,
		"sub":              account.AccountID,
		"dvid":             deviceId,
		"jti":              MakeID(),
		"creation_date":    time.Now().UTC().Format(time.RFC3339),
		"hours_expire":     hoursExpire,
		"token_session_id": global.SessionSettings["CurrentSessionId"],
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(tm.JWTSecret))
}

func (tm *TokenManager) CreateRefresh(account *models.Account, clientId, grantType, deviceId string, hoursExpire int) (string, error) {
	claims := jwt.MapClaims{
		"p":                MakeID(),
		"clid":             clientId,
		"t":                "refresh",
		"am":               grantType,
		"sub":              account.AccountID,
		"dvid":             deviceId,
		"jti":              MakeID(),
		"creation_date":    time.Now().UTC().Format(time.RFC3339),
		"hours_expire":     hoursExpire,
		"token_session_id": global.SessionSettings["CurrentSessionId"],
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(tm.JWTSecret))
}

func MakeID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}
