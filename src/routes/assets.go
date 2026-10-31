package routes

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

func InitAssets() {
	http.HandleFunc("/launcher/api/public/assets/", handlePublicAssets)
	http.HandleFunc("/launcher/api/public/distributionpoints/", handleDistributionPoints)
	http.HandleFunc("/Builds/Fortnite/Content/CloudDir/{n}", handleCloudDir)
}

func handleCloudDir(w http.ResponseWriter, r *http.Request) {
	const prefix = "/Builds/Fortnite/Content/CloudDir/"
	trimmedPath := strings.TrimPrefix(r.URL.Path, prefix)

	var fileName string
	if strings.HasSuffix(trimmedPath, ".manifest") {
		fileName = "Aventis.manifest"
	} else if strings.HasSuffix(trimmedPath, ".chunk") {
		fileName = "Aventis.chunk"
	} else if strings.HasSuffix(trimmedPath, ".ini") {
		fileName = "Full.ini"
	} else {
		http.Error(w, "Invalid file type", http.StatusBadRequest)
		return
	}

	filePath := "./static/CloudDir/" + fileName
	if _, err := os.Stat(filePath); err == nil {
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			http.Error(w, "Failed to read file", http.StatusInternalServerError)
			return
		}

		if fileName != "Full.ini" {
			w.Header().Set("Content-Type", "application/octet-stream")
		}
		w.WriteHeader(http.StatusOK)
		w.Write(fileContent)
		return
	}

	http.Error(w, "File not found", http.StatusNotFound)
}

func handleDistributionPoints(w http.ResponseWriter, r *http.Request) {
	distributions := []string{
		"https://download.epicgames.com/",
		"https://download2.epicgames.com/",
		"https://download3.epicgames.com/",
		"https://download4.epicgames.com/",
		"https://epicgames-download1.akamaized.net/",
	}

	response := map[string]interface{}{
		"distributions": distributions,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handlePublicAssets(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"appName":       "FortniteContentBuilds",
		"labelName":     "Aventis",
		"buildVersion":  "++Fortnite+Release-20.00-CL-19458861-Windows",
		"catalogItemId": "5cb97847cee34581afdbc445400e2f77",
		"expires":       "9999-12-31T23:59:59.999Z",
		"items": map[string]interface{}{
			"MANIFEST": map[string]interface{}{
				"signature":               "Aventis",
				"distribution":            "https://Aventis.ol.epicgames.com/",
				"path":                    "Builds/Fortnite/Content/CloudDir/Aventis.manifest",
				"hash":                    "55bb954f5596cadbe03693e1c06ca73368d427f3",
				"additionalDistributions": map[string]interface{}{},
			},
			"CHUNKS": map[string]interface{}{
				"signature":               "Aventis",
				"distribution":            "https://Aventis.ol.epicgames.com/",
				"path":                    "Builds/Fortnite/Content/CloudDir/Aventis.manifest",
				"additionalDistributions": map[string]interface{}{},
			},
		},
		"assetId": "FortniteContentBuilds",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
