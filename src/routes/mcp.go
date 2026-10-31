package routes

import (
	"go-backend/src/operations"
	"go-backend/src/services"
	"net/http"
	"strings"
)

func InitMCP() {
	http.HandleFunc("/fortnite/api/game/v2/profile/{accountId}/client/{operation}", HandleClientOperation)
	http.HandleFunc("/fortnite/api/game/v2/profile/{accountId}/dedicated_server/{operation}", HandleDedicatedClientOperation)
}

func HandleDedicatedClientOperation(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Path

	rawPath = strings.TrimPrefix(rawPath, "/fortnite/api/game/v2/")
	parts := strings.Split(rawPath, "/")
	if len(parts) < 4 {
		http.Error(w, "Invalid URL structure. Expected format: /profile/{accountId}/dedicated_server/{operation}", http.StatusBadRequest)
		return
	}

	accountId := parts[1]
	operation := parts[3]

	funcs := &services.Functions{}
	operations := &operations.Operations{}

	if accountId == "" || operation == "" {
		funcs.Log(services.WARNING, "WOW", "fucking emptyn")
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

	if operation == "QueryProfile" || operation == "BulkEquipBattleRoyaleCustomization" {
		operations.HandleQueryProfile(w, r)
		return
	}

	funcs.Log(services.WARNING, "WOW", "invalid operation: "+operation)
	funcs.CreateError(
		"errors.com.epicgames.common.operation_not_found",
		"The operation you requested could not be found",
		[]string{},
		1005,
		"any",
		"prod",
		"The operation you requested could not be found",
		"operation_not_found",
		w,
	)
}

func HandleClientOperation(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Path

	rawPath = strings.TrimPrefix(rawPath, "/fortnite/api/game/v2/")
	parts := strings.Split(rawPath, "/")
	if len(parts) < 4 {
		http.Error(w, "Invalid URL structure. Expected format: /profile/{accountId}/client/{operation}", http.StatusBadRequest)
		return
	}

	accountId := parts[1]
	operation := parts[3]

	funcs := &services.Functions{}
	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	operations := &operations.Operations{}

	error_ver := tokenCreation.VerifyToken(w, r)
	if error_ver == nil {
		return
	}

	if accountId == "" || operation == "" {
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

	if operation == "QueryProfile" || operation == "BulkEquipBattleRoyaleCustomization" || operation == "SetHardcoreModifier" {
		operations.HandleQueryProfile(w, r)
		return
	}

	if operation == "SetMtxPlatform" {
		operations.SetMtxPlatform(w, r)
		return
	}

	if operation == "ClientQuestLogin" {
		operations.ClientQuestLogin(w, r)
		return
	}

	if operation == "SetCosmeticLockerSlot" {
		operations.SetCosmeticLockerSlot(w, r)
		return
	}

	if operation == "PurchaseCatalogEntry" {
		operations.HandlePurchaseCatalogEntry(w, r)
		return
	}

	if operation == "EquipBattleRoyaleCustomization" {
		operations.EquipBattleRoyaleCustomization(w, r)
		return
	}

	if operation == "ExchangeGameCurrencyForBattlePassOffer" {
		operations.ExchangeGameCurrencyForBattlePassOffer(w, r)
		return
	}

	if operation == "MarkItemSeen" {
		operations.MarkItemSeen(w, r)
		return
	}

	if operation == "MarkNewQuestNotificationSent" {
		operations.MarkNewQuestNotificationSent(w, r)
		return
	}

	if operation == "RemoveGiftBox" {
		operations.RemoveGiftBox(w, r)
		return
	}

	if operation == "GiftCatalogEntry" {
		operations.HandleGiftCatalogEntry(w, r)
		return
	}

	funcs.Log(services.WARNING, "LogMCP", "invalid operation: "+operation)
	funcs.CreateError(
		"errors.com.epicgames.common.operation_not_found",
		"The operation you requested could not be found",
		[]string{},
		1005,
		"any",
		"prod",
		"The operation you requested could not be found",
		"operation_not_found",
		w,
	)
}
