package routes

import (
	ProxySettings "go-backend/src/definitions/proxy"
	"go-backend/src/services"
	"net/http"
)

var proxy = services.Proxy{}
var funcs = services.Functions{}
var proxySettings = ProxySettings.Settings

func InitRoutes() {
	InitApi()
	InitAuth()
	InitLightswitch()
	InitFortnite()
	InitMCP()
	InitCloudstorage()
	InitTimeline()
	InitDiscovery()
	//InitParties()
	InitGameSessions()
	InitMatchmaking()
	InitAssets()
	InitBeryllium() // anticheat server
	InitDashboard()
	InitEvents()
	InitFriends()
	InitReporting()
}

func proxyRequestAether(w http.ResponseWriter, r *http.Request) {
	proxy.ProxyRequest(w, r, proxySettings.AETHER) // set to AETHER_LOCAL if your localhosting ts or AETHER for prod
}

func yes(w http.ResponseWriter, r *http.Request) {
	proxy.ProxyRequest(w, r, "http://127.0.0.1:8080")
}

func proxyRequestMMS(w http.ResponseWriter, r *http.Request) {
	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenCreation.VerifyToken(w, r)
	if user == nil {
		return
	}

	proxy.ProxyRequest(w, r, proxySettings.MMS) // set to MMS_LOCAL if your localhosting ts or MMS for prod
}
