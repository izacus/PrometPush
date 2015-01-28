package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	"io/ioutil"
	"math"
	"net/http"
)

const GCM_ENDPOINT = "https://android.googleapis.com/gcm/send"
const PAGE_SIZE = 1000

type PushEvent struct {
	Id      uint64  `json:"id"`
	Cause   string  `json:"cause"`
	CauseEn string  `json:"causeEn"`
	Road    string  `json:"road"`
	RoadEn  string  `json:"roadEn"`
	Time    uint64  `json:"created"`
	Valid   uint64  `json:"validUntil"`
	Y_wgs   float64 `json:"y_wgs"`
	X_wgs   float64 `json:"x_wgs"`
}

type PushPayload struct {
	RegistrationIds []string `json:"registration_ids"`
	Data            struct {
		Events []PushEvent `json:"events"`
	} `json:"data"`
	DryRun bool `json:"dry_run"`
}

func PushDispatcher(eventIdsChannel <-chan []uint64, gcmApiKey string) {
	log.WithField("serverApiKey", gcmApiKey).Debug("Initializing dispatcher.")
	for {
		ids := <-eventIdsChannel
		log.WithField("ids", ids).Debug("New ids received.")

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
			payload := PushPayload{RegistrationIds: keys, DryRun: true}
			payload.Data.Events = data
			dispatchPayload(payload, gcmApiKey)
		}

		tx.Close()
		db.Close()
	}
}

func getData(tx *gorm.DB, ids []uint64) []PushEvent {
	events := make([]PushEvent, len(ids))

	for i := 0; i < len(ids); i++ {
		var event Dogodek
		tx.First(&event, ids[i])
		events[i] = PushEvent{Id: event.Id, Cause: event.Vzrok, CauseEn: event.VzrokEn, Road: event.Cesta, RoadEn: event.CestaEn, Time: event.Vneseno, Valid: event.VeljavnostDo, Y_wgs: event.Y_wgs, X_wgs: event.X_wgs}
	}

	return events
}

func dispatchPayload(payload PushPayload, gcmApiKey string) error {
	var json_data bytes.Buffer
	json.NewEncoder(&json_data).Encode(payload)
	log.WithField("payload", json_data.String()).Debug("Dispatching pushes.")

	request, _ := http.NewRequest("POST", GCM_ENDPOINT, &json_data)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", fmt.Sprintf("key=%s", gcmApiKey))

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		log.WithFields(log.Fields{"err": err, "data": json_data}).Error("Failed to send GCM package.")
		return err
	}

	defer response.Body.Close()
	body, _ := ioutil.ReadAll(response.Body)

	log.WithFields(log.Fields{"status": response.Status, "body": string(body)}).Info("Dispatch OK.")
	return nil
}
