package routes

import (
	"encoding/json"
	"fmt"
	"go-backend/src/global"
	"net/http"
	"sync"
	"time"
)

var (
	timelineData      []byte
	timelineDataMutex sync.RWMutex
)

func InitTimeline() {
	http.HandleFunc("/fortnite/api/calendar/v1/timeline", HandleTimeline)
}

func HandleTimeline(w http.ResponseWriter, r *http.Request) {
	timelineDataMutex.RLock()
	if time.Since(lastFetchedTime) < cacheDuration && timelineData != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(timelineData)
		timelineDataMutex.RUnlock()
		return
	}
	timelineDataMutex.RUnlock()

	timelineDataMutex.Lock()
	defer timelineDataMutex.Unlock()

	if time.Since(lastFetchedTime) < cacheDuration && timelineData != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(timelineData)
		return
	}

	memory := funcs.GetVersionInfo(r)
	activeEvents := []map[string]string{
		{
			"eventType":   fmt.Sprintf("EventFlag.Season%d", memory.Season),
			"activeUntil": "9999-01-01T00:00:00.000Z",
			"activeSince": "2020-01-01T00:00:00.000Z",
		},
		{
			"eventType":   fmt.Sprintf("EventFlag.%s", memory.Lobby),
			"activeUntil": "9999-01-01T00:00:00.000Z",
			"activeSince": "2020-01-01T00:00:00.000Z",
		},
	}

	states := []map[string]interface{}{
		{
			"validFrom":    "2020-01-01T00:00:00.000Z",
			"activeEvents": activeEvents,
			"state": map[string]interface{}{
				"activeStorefronts":  []interface{}{},
				"eventNamedWeights":  make(map[string]string),
				"seasonNumber":       memory.Season,
				"seasonTemplateId":   "AthenaSeason:athenaseason" + fmt.Sprint(memory.Season),
				"matchXpBonusPoints": 0,
				"seasonBegin":        "2020-01-01T13:00:00Z",
				"seasonEnd":          "2025-12-01T14:00:00Z",
				"seasonDisplayedEnd": "9999-01-01T07:30:00Z",
				"weeklyStoreEnd":     global.Catalog.Expiration.UTC().Format(time.RFC3339),
				"dailyStoreEnd":      global.Catalog.Expiration.UTC().Format(time.RFC3339),
				"stwEventStoreEnd":   "9999-01-01T00:00:00.000Z",
				"stwWeeklyStoreEnd":  "9999-01-01T00:00:00.000Z",
				"sectionStoreEnds": map[string]string{
					"Featured": global.Catalog.Expiration.UTC().Format(time.RFC3339),
				},
			},
		},
	}

	response := map[string]interface{}{
		"channels": map[string]interface{}{
			"client-matchmaking": map[string]interface{}{
				"states":      []interface{}{},
				"cacheExpire": "9999-01-01T00:00:00.000Z",
			},
			"client-events": map[string]interface{}{
				"states":      states,
				"cacheExpire": "9999-01-01T00:00:00.000Z",
			},
		},
		"eventsTimeOffsetHrs": 0,
		"cacheIntervalMins":   0,
		"currentTime":         time.Now().Format(time.RFC3339),
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}

	timelineData = jsonResponse
	lastFetchedTime = time.Now()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(timelineData)
}
