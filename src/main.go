package main

import (
	"context"
	"fmt"
	XMPP "go-backend/src/definitions/xmpp"
	"go-backend/src/discord"
	"go-backend/src/global"
	"go-backend/src/routes"
	"go-backend/src/services"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

func logAllRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		funcs := &services.Functions{}

		ip := funcs.GetRealIP(r)
		isVpn, err := funcs.IsVPN(ip)
		if err != nil {
			http.Error(w, "Error checking VPN status: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if isVpn {
			htmlBytes, err := ioutil.ReadFile("./frontend/home/vpn.html")
			if err != nil {
				http.Error(w, "Failed to load HTML file", http.StatusInternalServerError)
				return
			}

			htmlContent := string(htmlBytes)
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, htmlContent)

			return
		}

		if r.URL.String() == "/" {
			htmlBytes, err := ioutil.ReadFile("./frontend/home/servers.html")
			if err != nil {
				http.Error(w, "Failed to load HTML file", http.StatusInternalServerError)
				return
			}

			htmlContent := string(htmlBytes)
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, htmlContent)

			return
		}

		if global.DebugMissingEndpoints {
			funcs.Log(services.WARNING, "LogServer", "Missing Endpoint: "+r.Method+" "+r.URL.String())
		}

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
	})
}

func updatePlrs() {
	for {
		if global.XmppServerConnected {
			var xmpp = XMPP.XmppService{}
			xmpp.SendGetPlayerPayload()
			discord.UpdatePlayerStatus()
		}
		time.Sleep(time.Duration(global.PlayerUpdateCooldown) * time.Second)
	}
}

func main() {
	funcs := &services.Functions{}

	logFile, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		log.Fatal("Could not open log file:", err)
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)

	global.InitGlobals()
	global.GlobalVar = "wowie"

	timeNow := time.Now().Format("01/02/06 15:04:05")
	funcs.Log(services.INFO, "LogWindows", "Log file open, "+timeNow)
	funcs.Log(services.INFO, "LogToken", "Session Signature Completed")

	discord.StartBot()
	routes.InitRoutes()
	go updatePlrs() // update plrcount every 5s

	if !global.XmppServerConnected {
		funcs.Log(services.ERROR, "LogWindows", "Failed to connect to XMPP server!")
	}

	mux := http.NewServeMux()
	http.Handle("/", logAllRequests(mux))

	port := ":8080"
	funcs.Log(services.INFO, "LogWindows", "Server Started on 0.0.0.0:8080")

	err = global.MongoDBClient.Ping(context.Background(), nil)
	if err != nil {
		funcs.Log(services.ERROR, "DATABASE", "Failed to ping database!")
	}

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("Error starting server: ", err)
	}
}
