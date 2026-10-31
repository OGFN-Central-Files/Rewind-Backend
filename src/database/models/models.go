package models

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"
)

type Account struct {
	AccountID  string
	Username   string
	Email      string
	Password   string
	Created    time.Time
	Banned     bool
	DiscordID  *string
	Additional map[string]interface{}
}

func NewAccount(accountID, username, email, password string) *Account {
	return &Account{
		AccountID:  accountID,
		Username:   username,
		Email:      email,
		Password:   password,
		Created:    time.Now(),
		Banned:     false,
		Additional: make(map[string]interface{}),
	}
}

var ProfileTypes = []string{
	"athena", "common_core", "profile0", "theater0", "campaign", "collections", "common_public", "creative", "metadata", "outpost0",
}

type Profile struct {
	AccountID   string
	Created     time.Time
	ProfileData map[string]interface{}
}

func NewProfile(accountID string, profileDir string) (*Profile, error) {
	profileData := make(map[string]interface{})

	for _, profileType := range ProfileTypes {
		data, err := loadProfile(profileType, profileDir)
		if err != nil {
			return nil, fmt.Errorf("failed to load profile %s: %v", profileType, err)
		}

		data["accountId"] = accountID
		data["_id"] = accountID

		profileData[profileType] = data
	}

	return &Profile{
		AccountID:   accountID,
		Created:     time.Now(),
		ProfileData: profileData,
	}, nil
}

func loadProfile(profileType, profileDir string) (map[string]interface{}, error) {
	profilePath := filepath.Join(profileDir, profileType+".json")

	fileData, err := ioutil.ReadFile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile file: %v", err)
	}

	var profileData map[string]interface{}
	if err := json.Unmarshal(fileData, &profileData); err != nil {
		return nil, fmt.Errorf("failed to parse profile JSON: %v", err)
	}

	return profileData, nil
}
