package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	"math"
	"net/http"
	"time"
)

const GCM_ENDPOINT = "https://android.googleapis.com/gcm/send"
const PAGE_SIZE = 1000

type PushEvent struct {
	Id            uint64  `json:"id"`
	Cause         string  `json:"cause"`
	CauseEn       string  `json:"causeEn"`
	Road          string  `json:"road"`
	RoadEn        string  `json:"roadEn"`
	RoadPriority  int32   `json:"roadPriority"`
	IsBorderXsing bool    `json:"isBorderCrossing"`
	Time          uint64  `json:"created"`
	Valid         uint64  `json:"validUntil"`
	Y_wgs         float64 `json:"y_wgs"`
	X_wgs         float64 `json:"x_wgs"`
}

type PushPayload struct {
	RegistrationIds []string `json:"registration_ids"`
	Data            struct {
		Events []PushEvent `json:"events"`
	} `json:"data"`
	TimeToLive uint32 `json:"time_to_live"`
	DryRun     bool   `json:"dry_run"`
}

type PushResponse struct {
	Success      uint32 `json:"success"`
	Failure      uint32 `json:"failure"`
	CanonicalIds uint32 `json:"canonical_ids"`
	Results      []struct {
		MessageId      string `json:"message_id"`
		RegistrationId string `json:"registration_id"`
		Error          string `json:"error"`
	} `json:"results"`
}

func PushDispatcher(eventIdsChannel <-chan []uint64, gcmApiKey string) {
	log.WithField("serverApiKey", gcmApiKey).Debug("Initializing dispatcher.")
	for {
		ids := <-eventIdsChannel
		log.WithField("ids", ids).Debug("New ids received.")
		if len(ids) == 0 {
			continue
		}

		// Paginate apikeys on a page boundary due to GCM server limit
		db := GetDbConnection()
		tx := db.Begin()

		data := getData(tx, ids)

		var keyCount int
		tx.Model(&ApiKey{}).Count(&keyCount)
		pages := int(math.Ceil(float64(keyCount) / float64(PAGE_SIZE)))

		for page := 0; page < pages; page++ {
			// Get list of ApiKeys
			var keys []string
			tx.Limit(PAGE_SIZE).Offset(page*PAGE_SIZE).Model(&ApiKey{}).Pluck("key", &keys)
			payload := PushPayload{RegistrationIds: keys, TimeToLive: 7200, DryRun: false}
			payload.Data.Events = data
			dispatchPayload(tx, payload, gcmApiKey)
		}

		tx.Commit()
		db.Close()
	}
}

func getData(tx *gorm.DB, ids []uint64) []PushEvent {
	events := make([]PushEvent, len(ids))

	for i := 0; i < len(ids); i++ {
		var event Dogodek
		tx.First(&event, ids[i])
		events[i] = PushEvent{Id: event.Id, Cause: event.Vzrok, CauseEn: event.VzrokEn, Road: event.Cesta, RoadEn: event.CestaEn, IsBorderXsing: event.MejniPrehod, RoadPriority: event.PrioritetaCeste, Time: event.Vneseno, Valid: event.VeljavnostDo, Y_wgs: event.Y_wgs, X_wgs: event.X_wgs}
	}

	return events
}

func dispatchPayload(tx *gorm.DB, payload PushPayload, gcmApiKey string) error {
	var json_data bytes.Buffer
	json.NewEncoder(&json_data).Encode(payload)
	log.WithField("payload", json_data.String()).Debug("Dispatching pushes.")

	request, _ := http.NewRequest("POST", GCM_ENDPOINT, &json_data)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", fmt.Sprintf("key=%s", gcmApiKey))

	client := &http.Client{}

	// Set payload with exponential backoff
	retryCount := 5
	retrySecs := 10

	var response *http.Response
	var err error

	for {
		response, err = client.Do(request)
		if response.StatusCode > 499 && response.StatusCode < 600 {
			time.Sleep(time.Duration(retrySecs) * time.Second)
			retryCount = retryCount - 1
			retrySecs = retrySecs * 2
			continue
		}

		if err != nil {
			log.WithFields(log.Fields{"err": err, "data": json_data}).Error("Failed to send GCM package.")
			return err
		}

		if err == nil || retryCount <= 0 {
			break
		}
	}

	var pushResponse PushResponse
	defer response.Body.Close()
	dec := json.NewDecoder(response.Body)
	dec.Decode(&pushResponse)

	log.WithFields(log.Fields{"status": response.Status, "r": pushResponse}).Info("Dispatch OK.")
	processResponse(tx, payload.RegistrationIds, pushResponse)

	return nil
}

func processResponse(tx *gorm.DB, registrationIds []string, response PushResponse) {

	// Process canonical IDs and non-registered clients
	for i := 0; i < len(registrationIds); i++ {
		if len(response.Results[i].Error) > 0 {
			if response.Results[i].Error == "NotRegistered" || response.Results[i].Error == "InvalidRegistration" {
				log.WithField("apiKey", registrationIds[i]).Info("Removing not registered push key.")
				tx.Where("key = ?", registrationIds[i]).Delete(ApiKey{})
			} else {
				log.WithFields(log.Fields{"error": response.Results[i].Error}).Warn("Unknown push error.")
			}
		}

		if len(response.Results[i].RegistrationId) > 0 {
			// Replace our push key with a new one
			tx.Where("key = ?", registrationIds[i]).Delete(ApiKey{})

			key := ApiKey{Key: response.Results[i].RegistrationId, RegistrationTime: time.Now().Unix(), UserAgent: "From Google"}
			tx.Where("key = ?", response.Results[i].RegistrationId).FirstOrInit(&key)

			if tx.NewRecord(key) {
				tx.Save(key)
			}

			log.WithFields(log.Fields{"old": registrationIds[i], "new": response.Results[i].RegistrationId}).Info("Replacing GCM key with canonical version.")
		}
	}
}
