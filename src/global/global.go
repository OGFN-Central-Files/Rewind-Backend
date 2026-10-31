package global

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/mongo"
)

type VersionInfo struct {
	CurrentBuild       string `json:"currentBuild"`
	CurrentCL          string `json:"currentCL"`
	CurrentPublicBuild string `json:"CurrentPublicBuild"`
	CurrentSeason      string `json:"currentSeason"`
	FortniteRelease    string `json:"fortniteRelease"`
	BuildNum           int
}

type CatalogBase struct {
	RefreshHrs  int           `json:"refreshIntervalHrs"`
	DailyHrs    int           `json:"dailyPurchaseHrs"`
	FeaturedHrs int           `json:"featuredPurchaseHrs"`
	Expiration  time.Time     `json:"expiration"`
	Storefronts []interface{} `json:"storefronts"`
	Sections    interface{}   `json:"custom_sections"`
}

type TimelineBase struct {
	Channels            interface{} `json:"channels"`
	EventsTimeOffsetHrs int         `json:"eventsTimeOffsetHrs"`
	CacheIntervalMins   int         `json:"cacheIntervalMins"`
	CurrentTime         time.Time   `json:"currentTime"`
}

type IPUserInfo struct {
	AccountID    string
	Username     string
	AccessToken  string
	RefreshToken string
	ClientID     string
	DeviceID     string
}

type Session struct {
	SessionId                       string                 `json:"sessionId"`
	ServerAddress                   string                 `json:"serverAddress"`
	LastUpdated                     string                 `json:"lastUpdated"`
	PlaylistName                    int                    `json:"playlistName"`
	OwnerId                         string                 `json:"ownerId"`
	OwnerName                       string                 `json:"ownerName"`
	ServerName                      string                 `json:"serverName"`
	MaxPublicPlayers                int                    `json:"maxPublicPlayers"`
	MaxPrivatePlayers               int                    `json:"maxPrivatePlayers"`
	ShouldAdvertise                 bool                   `json:"shouldAdvertise"`
	AllowJoinInProgress             bool                   `json:"allowJoinInProgress"`
	IsDedicated                     bool                   `json:"isDedicated"`
	UsesStats                       bool                   `json:"usesStats"`
	AllowInvites                    bool                   `json:"allowInvites"`
	UsesPresence                    bool                   `json:"usesPresence"`
	AllowJoinViaPresence            bool                   `json:"allowJoinViaPresence"`
	AllowJoinViaPresenceFriendsOnly bool                   `json:"allowJoinViaPresenceFriendsOnly"`
	BuildUniqueId                   string                 `json:"buildUniqueId"`
	Attributes                      map[string]interface{} `json:"attributes"`
	ServerPort                      int                    `json:"serverPort"`
	OpenPublicPlayers               int                    `json:"openPublicPlayers"`
	OpenPrivatePlayers              int                    `json:"openPrivatePlayers"`
	SortWeight                      int                    `json:"sortWeight"`
	Started                         bool                   `json:"started"`
	PublicPlayers                   []string               `json:"publicPlayers"`
	PrivatePlayers                  []string               `json:"privatePlayers"`
	Stopped                         bool                   `json:"stopped"`
}

var (
	GlobalVar             string
	KeychainURL           = "https://raw.githubusercontent.com/Lawin0129/LawinServer/refs/heads/main/responses/keychain.json"
	CatalogURL            = "http://aether-public-service-prod.privateuser.xyz"
	TimelineURL           = "http://aether-public-service-prod.privateuser.xyz/fortnite/api/calendar/v1/timeline"
	XmppToken             = "Rewind-XMPP-21.00XAO"
	MongoDBClient         *mongo.Client
	AccesssTokens         map[string]string
	BerylliumClients      map[string]interface{}
	RefreshTokens         map[string]string
	CurrentVersionInfo    VersionInfo
	LimitedTimeMode       string
	AdditionalPlaylists   string
	Catalog               CatalogBase
	Timeline              TimelineBase
	TargetYear            string
	Discovery             map[string]interface{}
	Sessions              map[string]Session
	WebSocketConn         *websocket.Conn
	XmppServerConnected   = false
	DebugMissingEndpoints = false
	GiftReceivedUsers     map[string]interface{}
	LauncherTokens        map[string]interface{}
	PlayerCount           int64
	PlayerUpdateCooldown  = 10
	MatchmakingSettings   = map[string]interface{}{
		"bHandleExternalJoinRequest":  false,
		"bEnableFindPlayerEndpoint":   false,
		"bForceBeaconPort":            false,
		"bMustHaveBerylliumConnected": true, // disabling this will stop forcing anticheat being connected.
		"bMustHaveHwidVerified":       true, // disabling this will also disable hwid checks.
	}
	SessionSettings = map[string]interface{}{
		"bAllowSessionAfterRestart": false, // allows previous sessions to still login, even after backend restarted.
		"CurrentSessionId":          "",    // server will automatically set this
	}
	TestingSettings = map[string]interface{}{
		"bTestingPreviligedOnly": true, // Private/Public testing or prod
		"GuildBypassPasswords":   "gs",
		"TestingGuildId":         "1380268084237045880",
	}
	LauncherSettings = map[string]interface{}{
		"bIsLauncherOnline": true,
		"DownloadReference": "https://public-service-live.ol.privateuser.xyz/Rewind_0.0.8_x64-setup.exe",
		"LauncherVersion":   "0.0.8",
	}
	DedicatedSettings = map[string]interface{}{
		"VBucksPerKill": 150,
	}
	Mu            = sync.RWMutex{}
	IPToUserMap   = make(map[string]IPUserInfo)
	IPToUserMutex = sync.Mutex{}
)

func cleanupTokens() {
	for {
		time.Sleep(24 * time.Hour)
		AccesssTokens = make(map[string]string)
		RefreshTokens = make(map[string]string)
		LauncherTokens = make(map[string]interface{})
		log.Println("Launcher Tokens cleaned up!")
	}
}

func InitGlobals() {
	TargetYear = "wow" // set this so it will start rolling back storefront (this is very unproper)
	AccesssTokens = make(map[string]string)
	RefreshTokens = make(map[string]string)
	GiftReceivedUsers = make(map[string]interface{})
	LauncherTokens = make(map[string]interface{})
	BerylliumClients = make(map[string]interface{})
	Sessions = make(map[string]Session)
	Mu = sync.RWMutex{}

	SessionSettings["CurrentSessionId"] = strings.ReplaceAll(uuid.New().String(), "-", "")

	CurrentVersionInfo = VersionInfo{
		CurrentBuild:       "24.20",
		CurrentCL:          "24939793",
		CurrentSeason:      "24",
		BuildNum:           24.00,
		CurrentPublicBuild: "24.2",
	}

	if CurrentVersionInfo.BuildNum == 15.00 {
		AdditionalPlaylists = `
[/Script/FortniteGame.FortGameInstance]
!FrontEndPlaylistData=ClearArray
+FrontEndPlaylistData=(PlaylistName=Playlist_DefaultSolo, PlaylistAccess=(bEnabled=True, bIsDefaultPlaylist=true, bVisibleWhenDisabled=false, bDisplayAsNew=false, CategoryIndex=0, bDisplayAsLimitedTime=false, DisplayPriority=1))
+FrontEndPlaylistData=(PlaylistName=Playlist_DefaultDuo, PlaylistAccess=(bEnabled=True, bIsDefaultPlaylist=false, bVisibleWhenDisabled=false, bDisplayAsNew=false, CategoryIndex=0, bDisplayAsLimitedTime=false, DisplayPriority=2))
+FrontEndPlaylistData=(PlaylistName=Playlist_DefaultSquad, PlaylistAccess=(bEnabled=True, bIsDefaultPlaylist=false, bVisibleWhenDisabled=false, bDisplayAsNew=false, CategoryIndex=0, bDisplayAsLimitedTime=false, DisplayPriority=3))`
	} else if CurrentVersionInfo.BuildNum == 9.00 {
		AdditionalPlaylists = `
[/Script/FortniteGame.FortGameInstance]
!FrontEndPlaylistData=ClearArray
+FrontEndPlaylistData=(PlaylistName=Playlist_DefaultSolo, PlaylistAccess=(bEnabled=True, bIsDefaultPlaylist=true, bVisibleWhenDisabled=false, bDisplayAsNew=false, CategoryIndex=0, bDisplayAsLimitedTime=false, DisplayPriority=1))`
	}

	CurrentVersionInfo.FortniteRelease = "FortniteRelease-" + CurrentVersionInfo.CurrentBuild + "-CL-" + CurrentVersionInfo.CurrentCL + "-Windows"

	err := fetchCatalogData()
	if err != nil {
		log.Fatalf("Error fetching catalog data: %v", err)
	}

	go refreshCatalogOnExpiration() // refresh catalog when expired
	go cleanupTokens()              // cleanup tokens every 8hrs
	connectWebSocket()
}

func connectWebSocket() {
	var err error

	WebSocketConn, _, err = websocket.DefaultDialer.Dial("wss://xmpp-admin.privateuser.xyz/xmpp/admin/ws", nil) // combine protocol, adr, endpoint & dial
	if err == nil {
		XmppServerConnected = true
		go listenForDisconnects()
	}
}

func listenForDisconnects() {
	for {
		_, msg, err := WebSocketConn.ReadMessage()
		if err != nil {
			XmppServerConnected = false // dont remove this thanks
			reconnectWebSocket()
			break
		} else {
			type XmppMessage struct {
				Payload string `json:"payload"`
				Amount  string `json:"amount"`
			}

			var m XmppMessage
			err := json.Unmarshal([]byte(msg), &m)
			if err != nil {
				return
			}

			if m.Payload == "PlayerUpload" {
				playerCount, _ := strconv.ParseInt(m.Amount, 10, 64)
				PlayerCount = playerCount
			}
		}
	}
}

func reconnectWebSocket() {
	for {
		connectWebSocket()
		if WebSocketConn != nil {
			break
		}

		time.Sleep(5 * time.Second)
	}
}

func LoadDiscovery() error {
	filePath := "./static/responses/discovery_frontend.json"
	contentBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading request: %v", err)
	}

	err = json.Unmarshal(contentBytes, &Discovery)
	if err != nil {
		return fmt.Errorf("error unmarshalling JSON response: %v", err)
	}

	return nil
}

func LoadTimelineBase() error {
	resp, err := http.Get(TimelineURL)
	if err != nil {
		return fmt.Errorf("error sending GET request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-OK HTTP status: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %v", err)
	}

	err = json.Unmarshal(body, &Timeline)
	if err != nil {
		return fmt.Errorf("error unmarshalling JSON response: %v", err)
	}

	return nil
}

func fetchCatalogData() error {
	currentDate := time.Now()
	month := currentDate.Month()
	day := currentDate.Day()

	const epsilon = 0.000001
	if TargetYear == "wow" {
		CurrentBuild, err := strconv.ParseFloat(CurrentVersionInfo.CurrentBuild, 64)
		if err != nil {
			log.Printf("my current season doesnt work hlep: %v", err)
		} else {
			if CurrentBuild <= 2.10+epsilon {
				TargetYear = "2017"
			} else if CurrentBuild <= 7.10+epsilon {
				TargetYear = "2018"
			} else if CurrentBuild <= 12.00+epsilon {
				TargetYear = "2019"
			} else if CurrentBuild <= 15.20+epsilon {
				TargetYear = "2020"
			} else if CurrentBuild <= 15.20+epsilon {
				TargetYear = "2020"
			} else if CurrentBuild <= 20.00+epsilon {
				TargetYear = "2021"
			} else if CurrentBuild <= 24.00+epsilon {
				TargetYear = "2022"
			} else if CurrentBuild <= 28.00+epsilon {
				TargetYear = "2023"
			}
		}

		if CurrentBuild <= 19.10+epsilon {
			newDate := currentDate.AddDate(0, 2, 0)
			month = newDate.Month()
			day = newDate.Day()
		}
	}

	url := fmt.Sprintf(
		"%s/get?apiKey=c060ae1f-26fc-4922-ac2f-22a281d4a4c3&year=%s&month=%02d&day=%02d",
		CatalogURL, TargetYear, month, day,
	)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error sending GET request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-OK HTTP status: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %v", err)
	}

	err = json.Unmarshal(body, &Catalog)
	if err != nil {
		return fmt.Errorf("error unmarshalling JSON response: %v", err)
	}

	err_timeline := LoadTimelineBase()
	if err_timeline != nil {
		return fmt.Errorf("error unmarshalling JSON response (timeline): %v", err_timeline)
	}

	err_discovery := LoadDiscovery()
	if err_discovery != nil {
		return fmt.Errorf("error unmarshalling JSON response (discovery): %v", err_timeline)
	}

	return nil
}

func refreshCatalogOnExpiration() {
	for {
		if time.Now().After(Catalog.Expiration) {
			log.Println("Catalog expired. Fetching new data...")
			err := fetchCatalogData()
			if err != nil {
				log.Printf("Error fetching new catalog data: %v", err)
			}
			time.Sleep(time.Until(Catalog.Expiration))
		} else {
			time.Sleep(time.Minute)
		}
	}
}
