package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"io/ioutil"
	"net/http"
)

const GCM_ENDPOINT = "https://android.googleapis.com/gcm/send"

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

		payload := PushPayload{RegistrationIds: []string{"id1", "id2"}, DryRun: true}
		var json_data bytes.Buffer
		json.NewEncoder(&json_data).Encode(payload)

		request, _ := http.NewRequest("POST", GCM_ENDPOINT, &json_data)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", fmt.Sprintf("key=%s", gcmApiKey))

		client := &http.Client{}
		response, err := client.Do(request)
		if err != nil {
			log.WithFields(log.Fields{"err": err, "data": json_data}).Error("Failed to send GCM package.")
			continue
		}

		defer response.Body.Close()
		body, _ := ioutil.ReadAll(response.Body)

		log.WithFields(log.Fields{"status": response.Status, "body": string(body)}).Info("Dispatch OK.")
	}
}
