package routes

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

func InitDiscovery() {
	http.HandleFunc("/fortnite/api/game/v2/creative/discovery/surface/", proxyRequestAether)
	http.HandleFunc("/api/v2/discovery/surface/", proxyRequestAether)
	http.HandleFunc("/api/v1/discovery/surface/", proxyRequestAether)
	http.HandleFunc("/fortnite/api/discovery/accessToken/", proxyRequestAether)
	http.HandleFunc("/links/api/fn/mnemonic", proxyRequestAether)
	http.HandleFunc("/links/api/fn/mnemonic/", proxyRequestAether)
	http.HandleFunc("/api/v1/links/lock-status/", proxyRequestAether)
	http.HandleFunc("/api/v1/assets/Fortnite/", proxyRequestAether)

	http.HandleFunc("/api/v1/fortnite-br/surfaces/{surface}/target", HandleMotdSurface)
}

type ContentItem struct {
	ContentFields map[string]map[string]string `json:"contentFields"`
}

type MOTD struct {
	ContentItems []ContentItem `json:"contentItems"`
}

type RequestBodyMOTD struct {
	Language string `json:"language"`
}

func HandleMotdSurface(w http.ResponseWriter, r *http.Request) {
	filePath := "./static/responses/motd.json"
	contentBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		funcs.CreateErrorV2(
			"errors.com.epicgames.common.internal_server_error",
			"Failed to find bytes.",
			[]string{}, -1, "", 500, w,
		)
		return
	}

	var contentPages map[string]interface{}
	err = json.Unmarshal(contentBytes, &contentPages)
	if err != nil {
		funcs.CreateErrorV2(
			"errors.com.epicgames.common.internal_server_error",
			"Failed to decode bytes.",
			[]string{}, -1, "", 500, w,
		)
		return
	}

	var reqBody RequestBodyMOTD
	err = json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		funcs.CreateErrorV2(
			"errors.com.epicgames.common.internal_server_error",
			"Failed to decode body.",
			[]string{}, -1, "", 500, w,
		)
		return
	}

	fields := []string{"title", "body", "TeaserTitle", "FullScreenTitle", "FullScreenBody"}

	if items, ok := contentPages["contentItems"].([]interface{}); ok {
		for _, rawItem := range items {
			if item, ok := rawItem.(map[string]interface{}); ok {
				if contentFields, ok := item["contentFields"].(map[string]interface{}); ok {
					for _, field := range fields {
						if langMap, ok := contentFields[field].(map[string]interface{}); ok {
							if val, ok := langMap[reqBody.Language]; ok {
								contentFields[field] = val
							}
						}
					}
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(contentPages); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}
