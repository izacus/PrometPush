package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"net/http"
	"time"

	"github.com/getsentry/raven-go"
	"github.com/jinzhu/gorm"
	log "github.com/sirupsen/logrus"
)

const GCM_ENDPOINT = "https://android.googleapis.com/gcm/send"
const PAGE_SIZE = 1000

type PushEvent struct {
	Id            int64   `json:"id"`
	Cause         string  `json:"cause"`
	CauseEn       string  `json:"causeEn"`
	Road          string  `json:"road"`
	RoadEn        string  `json:"roadEn"`
	RoadPriority  int32   `json:"roadPriority"`
	Description   string  `json:"description"`
	DescriptionEn string  `json:"descriptionEn"`
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

func PushDispatcher(eventIdsChannel <-chan []string, gcmApiKey string) {
	log.WithField("serverApiKey", gcmApiKey).Debug("Initializing dispatcher.")

	// Paginate apikeys on a page boundary due to GCM server limit
	db := GetDbConnection()
	for {
		ids := <-eventIdsChannel
		log.WithField("ids", ids).Debug("New ids received.")
		if len(ids) == 0 {
			continue
		}

		tx := db.Begin()
		data := getData(tx, ids)
		if data == nil {
			log.Error("Failed to retrieve data for passed ids")
			tx.Rollback()
			return
		}

		var keyCount int
		if err := tx.Model(&ApiKey{}).Count(&keyCount).Error; err != nil {
			log.WithField("error", err).Error("Failed to check existence of an event.")
			raven.CaptureErrorAndWait(err, nil)
			continue
		}

		pages := int(math.Ceil(float64(keyCount) / float64(PAGE_SIZE)))

		for page := 0; page < pages; page++ {
			// Get list of ApiKeys
			var keys []string
			if err := tx.Limit(PAGE_SIZE).Offset(page*PAGE_SIZE).Model(&ApiKey{}).Pluck("key", &keys).Error; err != nil {
				log.WithField("error", err).Error("Failed load device tokens")
				raven.CaptureErrorAndWait(err, nil)
				continue
			}

			log.WithField("num", len(keys)).Info("Dispatching payload...")
			payload := PushPayload{RegistrationIds: keys, TimeToLive: 7200, DryRun: false}
			payload.Data.Events = data
			dispatchPayload(tx, payload, gcmApiKey)
		}

		tx.Commit()
	}
}

func getData(tx *gorm.DB, ids []string) []PushEvent {
	events := make([]PushEvent, len(ids))

	for i := 0; i < len(ids); i++ {
		var event Dogodek
		if err := tx.First(&event, "id = ?", ids[i]).Error; err != nil {
			log.WithFields(log.Fields{"id": ids[i]}).Error("Failed to retrieve event data for dispatch")
			raven.CaptureErrorAndWait(err, nil)
			return nil
		}

		var desc string
		var descEn string

		// Devices won't show description in the notification if
		// there's more than one incoming so include it only when there's
		// a single event. This mainly prevents going over the push payload size.
		if len(ids) == 1 {
			desc = event.Opis
			descEn = event.OpisEn
		} else {
			desc = ""
			descEn = ""
		}

		// Calculate Id hash
		algo := fnv.New32a()
		algo.Write([]byte(event.Id))
		id_hash := int64(algo.Sum32())

		events[i] = PushEvent{Id: id_hash,
			Cause:         event.Vzrok,
			CauseEn:       event.VzrokEn,
			Road:          event.Cesta,
			RoadEn:        event.CestaEn,
			IsBorderXsing: event.MejniPrehod,
			RoadPriority:  event.PrioritetaCeste,
			Time:          event.Updated * 1000, // Need to convert to milliseconds
			Valid:         event.VeljavnostDo * 1000,
			Description:   desc,
			DescriptionEn: descEn,
			Y_wgs:         event.Y_wgs,
			X_wgs:         event.X_wgs}
	}

	return events
}

func dispatchPayload(tx *gorm.DB, payload PushPayload, gcmApiKey string) error {
	log.Debug("Dispatching...")

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
		GetStatistics().Dispatches++
		if response.StatusCode > 499 && response.StatusCode < 600 {
			time.Sleep(time.Duration(retrySecs) * time.Second)
			retryCount = retryCount - 1
			retrySecs = retrySecs * 2

			raven.CaptureMessage(response.Status, nil)
			GetStatistics().FailedDispatches++
			continue
		}

		if err != nil {
			log.WithFields(log.Fields{"err": err, "data": json_data}).Error("Failed to send GCM package.")
			GetStatistics().FailedDispatches++
			raven.CaptureErrorAndWait(err, nil)
			return err
		}

		if response.StatusCode > 399 && response.StatusCode < 500 {
			GetStatistics().FailedDispatches++
			log.WithFields(log.Fields{"response": response.Status}).Error("Failed to dispatch notifications!")
			raven.CaptureMessage(response.Status, nil)
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
				if err := tx.Where("key = ?", registrationIds[i]).Delete(ApiKey{}).Error; err != nil {
					raven.CaptureErrorAndWait(err, nil)
				}
			} else if response.Results[i].Error == "MissingRegistration" {
				raven.CaptureMessage("Got MissingRegistration", map[string]string{"registrationId": registrationIds[i]})
				if err := tx.Where("key = ?", registrationIds[i]).Delete(ApiKey{}).Error; err != nil {
					raven.CaptureErrorAndWait(err, nil)
				}
			} else {
				log.WithFields(log.Fields{"error": response.Results[i].Error}).Warn("Unknown push error.")
				raven.CaptureMessage("Unknown push error.", map[string]string{"error": response.Results[i].Error, "registrationId": registrationIds[i]})
			}
		}

		if len(response.Results[i].RegistrationId) > 0 {
			// Replace our push key with a new one
			if err := tx.Where("key = ?", registrationIds[i]).Delete(ApiKey{}).Error; err != nil {
				raven.CaptureErrorAndWait(err, nil)
			}

			key := ApiKey{Key: response.Results[i].RegistrationId, RegistrationTime: time.Now().Unix(), UserAgent: "From Google"}
			if err := tx.Where("key = ?", response.Results[i].RegistrationId).FirstOrInit(&key).Error; err != nil {
				raven.CaptureErrorAndWait(err, nil)
			}

			if tx.NewRecord(key) {
				if err := tx.Save(key).Error; err != nil {
					raven.CaptureErrorAndWait(err, nil)
				}
			}

			GetStatistics().UpdatedPushKeys++
			log.WithFields(log.Fields{"old": registrationIds[i], "new": response.Results[i].RegistrationId}).Info("Replacing GCM key with canonical version.")
		}
	}
}
