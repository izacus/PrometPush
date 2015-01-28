package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"io/ioutil"
	"math"
	"net/http"
)

const GCM_ENDPOINT = "https://android.googleapis.com/gcm/send"
const PAGE_SIZE = 1000

type PushPayload struct {
	RegistrationIds []string `json:"registration_ids"`
	Data            struct{} `json:"data"`
	DryRun          bool     `json:"dry_run"`
}

func PushDispatcher(eventIdsChannel <-chan []uint64, gcmApiKey string) {
	log.WithField("serverApiKey", gcmApiKey).Debug("Initializing dispatcher.")
	for {
		ids := <-eventIdsChannel
		log.WithField("ids", ids).Debug("New ids received.")

		// Paginate apikeys on a page boundary due to GCM server limit
		db := GetDbConnection()
		tx := db.Begin()

		var keyCount int
		tx.Model(&ApiKey{}).Count(&keyCount)
		pages := int(math.Ceil(float64(keyCount) / float64(PAGE_SIZE)))

		for page := 0; page < pages; page++ {
			// Get list of ApiKeys
			var keys []string
			tx.Limit(PAGE_SIZE).Offset(page*PAGE_SIZE).Model(&ApiKey{}).Pluck("key", &keys)
			payload := PushPayload{RegistrationIds: keys, DryRun: true}
			dispatchPayload(payload, gcmApiKey)
		}

		tx.Close()
		db.Close()
	}
}

func dispatchPayload(payload PushPayload, gcmApiKey string) error {
	var json_data bytes.Buffer
	json.NewEncoder(&json_data).Encode(payload)

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
