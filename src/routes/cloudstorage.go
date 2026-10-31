package routes

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go-backend/src/services"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func InitCloudstorage() {
	http.HandleFunc("/fortnite/api/cloudstorage/user/", CloudStorageUserHandler)
	http.HandleFunc("/fortnite/api/cloudstorage/system", CloudStorageSystemHandler)
	http.HandleFunc("/fortnite/api/cloudstorage/system/", CloudStorageSystemFileHandler)
	http.HandleFunc("/fortnite/api/cloudstorage/system/config", CloudstorageConfigReq)
}

func CloudstorageConfigReq(w http.ResponseWriter, r *http.Request) {
	tokenCreation := services.NewTokenManager("JWT_TOKEN_1231289SNN")
	user := tokenCreation.VerifyToken(w, r)
	if user == nil {
		return
	}

	filePath := "./static/responses/cloud_config.json"
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "Invalid file name", http.StatusNotFound)
		return
	}

	var config map[string]interface{}
	err = json.Unmarshal(contentBytes, &config)
	if err != nil {
		http.Error(w, "Invalid file name", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(config); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}

func CloudStorageUserHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		CloudStorageUserGetHandler(w, r)
	case http.MethodPut:
		CloudStorageUserPutHandler(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func CloudStorageUserGetHandler(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimPrefix(r.URL.Path, "/fortnite/api/cloudstorage/user/")
	if strings.Contains(accountID, "/") {
		accountID = strings.Split(accountID, "/")[0]
	}

	memory := funcs.GetVersionInfo(r)
	clientSettingsPath := "./static/clientsettings/" + accountID + "/Season" + strconv.Itoa(memory.Season)

	if _, err := os.Stat(clientSettingsPath); os.IsNotExist(err) {
		os.MkdirAll(clientSettingsPath, 0755)
	}

	filePath := clientSettingsPath + "/ClientSettings.Sav"

	if _, err := os.Stat(filePath); err == nil {
		parsed, err := os.ReadFile(filePath)
		if err != nil {
			http.Error(w, "Failed to read file", http.StatusInternalServerError)
			return
		}

		sha1Hash := sha1.New()
		sha1Hash.Write(parsed)
		sha1HashString := fmt.Sprintf("%x", sha1Hash.Sum(nil))

		sha256Hash := sha256.New()
		sha256Hash.Write(parsed)
		sha256HashString := fmt.Sprintf("%x", sha256Hash.Sum(nil))

		fileStats, _ := os.Stat(filePath)

		response := []map[string]interface{}{
			{
				"uniqueFilename": "ClientSettings.Sav",
				"filename":       "ClientSettings.Sav",
				"hash":           sha1HashString,
				"hash256":        sha256HashString,
				"length":         len(parsed),
				"contentType":    "application/octet-stream",
				"uploaded":       fileStats.ModTime(),
				"storageType":    "S3",
				"storageIds":     map[string]interface{}{},
				"accountId":      accountID,
				"doNotCache":     false,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode([]interface{}{})
}

func CloudStorageUserPutHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/fortnite/api/cloudstorage/user/"), "/")
	if len(parts) != 2 {
		http.Error(w, "Invalid path format", http.StatusBadRequest)
		return
	}

	accountID := parts[0]
	fileName := parts[1]

	if fileName != "ClientSettings.Sav" {
		http.Error(w, "Invalid file name", http.StatusBadRequest)
		return
	}

	memory := funcs.GetVersionInfo(r)
	clientSettingsPath := "./static/clientsettings/" + accountID + "/Season" + strconv.Itoa(memory.Season)

	if _, err := os.Stat(clientSettingsPath); os.IsNotExist(err) {
		if err := os.MkdirAll(clientSettingsPath, 0755); err != nil {
			http.Error(w, "Failed to create directory", http.StatusInternalServerError)
			return
		}
	}

	filePath := clientSettingsPath + "/ClientSettings.Sav"

	fileData, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	if err := os.WriteFile(filePath, fileData, 0644); err != nil {
		http.Error(w, "Failed to write file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func CloudStorageSystemHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		dir := "./static/hotfixes"

		if _, err := os.Stat(dir); os.IsNotExist(err) {
			err := os.MkdirAll(dir, 0755)
			if err != nil {
				http.Error(w, "Failed to create hotfixes directory", http.StatusInternalServerError)
				return
			}
		}

		files, err := os.ReadDir(dir)
		if err != nil {
			http.Error(w, "Failed to read hotfixes directory", http.StatusInternalServerError)
			return
		}

		var cloudStorage []map[string]interface{}

		for _, file := range files {
			parsed, err := os.ReadFile(dir + "/" + file.Name())
			if err != nil {
				continue
			}

			sha1Hash := sha1.New()
			sha1Hash.Write(parsed)
			sha1HashString := fmt.Sprintf("%x", sha1Hash.Sum(nil))

			sha256Hash := sha256.New()
			sha256Hash.Write(parsed)
			sha256HashString := fmt.Sprintf("%x", sha256Hash.Sum(nil))

			info, err := file.Info()
			if err != nil {
				funcs.Log(services.ERROR, "LogCloud", "Failed to get FileInfo | cloudstorage.go::199")
				return
			}

			cloudStorage = append(cloudStorage, map[string]interface{}{
				"uniqueFilename": file.Name(),
				"filename":       file.Name(),
				"hash":           sha1HashString,
				"hash256":        sha256HashString,
				"length":         len(parsed),
				"contentType":    "application/octet-stream",
				"uploaded":       info.ModTime(),
				"storageType":    "S2",
				"storageIds":     map[string]interface{}{},
				"doNotCache":     true,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(cloudStorage)
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}

func CloudStorageSystemFileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	fileName := strings.TrimPrefix(r.URL.Path, "/fortnite/api/cloudstorage/system/")
	if strings.Contains(fileName, "..") || strings.Contains(fileName, "~") || fileName == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	filePath := filepath.Join("./static/hotfixes/", fileName)

	if _, err := os.Stat(filePath); err == nil {
		content, err := os.ReadFile(filePath)
		if err != nil {
			http.Error(w, "Failed to read file", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(content)
		return
	}

	w.WriteHeader(http.StatusOK)
}
