package ProxySettings

type ProxySettings struct {
	AETHER       string
	AETHER_LOCAL string
	MMS          string
	MMS_LOCAL    string
}

var Settings = ProxySettings{
	AETHER:       "http://aether-public-service-prod.privateuser.xyz",
	AETHER_LOCAL: "http://localhost:20039",
	MMS:          "https://matchmaking-service-live.privateuser.xyz",
	MMS_LOCAL:    "http://192.168.0.227:3551",
}
