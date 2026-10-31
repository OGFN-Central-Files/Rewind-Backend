package routes

import (
	"encoding/json"
	"net/http"
)

func InitLightswitch() {
	http.HandleFunc("/lightswitch/api/service/bulk/status", StatusServiceBulk)
}

type LauncherInfoDTO struct {
	AppName       string `json:"appName"`
	CatalogItemID string `json:"catalogItemId"`
	Namespace     string `json:"namespace"`
}

type ServiceStatus struct {
	ServiceInstanceID  string          `json:"serviceInstanceId"`
	Status             string          `json:"status"`
	Message            string          `json:"message"`
	MaintenanceURI     *string         `json:"maintenanceUri"`
	OverrideCatalogIDs []string        `json:"overrideCatalogIds"`
	AllowedActions     []string        `json:"allowedActions"`
	Banned             bool            `json:"banned"`
	LauncherInfoDTO    LauncherInfoDTO `json:"launcherInfoDTO"`
}

func StatusServiceBulk(w http.ResponseWriter, r *http.Request) {
	status := "UP"
	message := "fortnite is up."

	response := []ServiceStatus{{
		ServiceInstanceID:  "fortnite",
		Status:             status,
		Message:            message,
		MaintenanceURI:     nil,
		OverrideCatalogIDs: []string{"a7f138b2e51945ffbfdacc1af0541053"},
		AllowedActions:     []string{"PLAY", "DOWNLOAD"},
		Banned:             false,
		LauncherInfoDTO: LauncherInfoDTO{
			AppName:       "Fortnite",
			CatalogItemID: "4fe75bbc5a674f4f9b356b5c90567da5",
			Namespace:     "fn",
		},
	}}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
