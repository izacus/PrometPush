package main

import (
	"bytes"
	"context"
	"encoding/json"
	"hash/fnv"
	"math"
	"time"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/messaging"
	"google.golang.org/api/option"

	_ "firebase.google.com/go"
	"github.com/getsentry/sentry-go"
	"github.com/jinzhu/gorm"
	log "github.com/sirupsen/logrus"
)

const pageSize = 99

// PushEvent describes a single event happening on the road.
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

type pushPayload struct {
	RegistrationIds []string
	Events          []PushEvent
}

// PushDispatcher handles dispatching of notifications to the GCM server. The notifications are coming from the channel
// listed.
func PushDispatcher(eventIdsChannel <-chan []string, firebaseConfigurationJSONFile string, individualPushEnabled bool) {
	log.WithField("serverApiKey", firebaseConfigurationJSONFile).Debug("Initializing dispatcher.")

	// Paginate apikeys on a page boundary due to GCM server limit
	db := GetDbConnection()

	opt := option.WithCredentialsFile(firebaseConfigurationJSONFile)
	ctx := context.Background()

	var app *firebase.App
	var client *messaging.Client
	var err error

	if app, err = firebase.NewApp(ctx, nil, opt); err != nil {
		log.WithField("error", err).Fatal("Failed to initialize firebase SDK.")
	}

	if client, err = app.Messaging(ctx); err != nil {
		log.WithField("error", err).Fatal("Failed to initialize firebase client.")
	}

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

		dispatchPayloadToTopic(ctx, data, client)

		if individualPushEnabled {
			var keyCount int
			if err := tx.Model(&ApiKey{}).Count(&keyCount).Error; err != nil {
				log.WithField("error", err).Error("Failed to check existence of an event.")
				sentry.CaptureException(err)
				continue
			}

			pages := int(math.Ceil(float64(keyCount) / float64(pageSize)))

			for page := 0; page < pages; page++ {
				// Get list of ApiKeys
				var keys []string
				if err := tx.Limit(pageSize).Offset(page*pageSize).Model(&ApiKey{}).Pluck("key", &keys).Error; err != nil {
					log.WithField("error", err).Error("Failed load device tokens")
					sentry.CaptureException(err)
					continue
				}

				log.WithField("num", len(keys)).Info("Dispatching payload...")
				payload := pushPayload{RegistrationIds: keys}
				payload.Events = data
				dispatchPayload(ctx, tx, payload, client)
			}
		}

		tx.Commit()
	}
}

func getData(tx *gorm.DB, ids []string) []PushEvent {
	// There's a payload limit on FCM so we need to make sure we don't send too many.
	if len(ids) > 10 {
		ids = ids[len(ids)-10:]
	}

	events := make([]PushEvent, len(ids))
	for i := 0; i < len(ids); i++ {
		var event Dogodek
		if err := tx.First(&event, "id = ?", ids[i]).Error; err != nil {
			log.WithFields(log.Fields{"id": ids[i]}).Error("Failed to retrieve event data for dispatch")
			sentry.CaptureException(err)
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
		idHash := int64(algo.Sum32())

		events[i] = PushEvent{Id: idHash,
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

func dispatchPayloadToTopic(ctx context.Context, events []PushEvent, client *messaging.Client) {
	log.Debug("Dispatching to topics...")
	var jsonData bytes.Buffer
	if err := json.NewEncoder(&jsonData).Encode(events); err != nil {
		log.WithField("error", err).Error("Failed to encode JSON payload for dispatch.")
		sentry.CaptureException(err)
		return
	}

	var ttl = time.Duration(2) * time.Hour
	message := &messaging.Message{
		Data: map[string]string{
			"events": jsonData.String(),
		},
		Topic: "allRoadEvents",
		Android: &messaging.AndroidConfig{
			TTL: &ttl,
		},
	}

	retryCount := 5
	retrySecs := 10

	var err error
	for {
		if DebugMode {
			_, err = client.SendDryRun(ctx, message)
		} else {
			_, err = client.Send(ctx, message)
		}

		if err == nil {
			break
		}

		if retryCount <= 0 {
			break
		}

		log.WithFields(log.Fields{"err": err, "data": json_data}).Error("Failed to send topic package.")
		sentry.CaptureException(err)
		time.Sleep(time.Duration(retrySecs) * time.Second)
		retryCount = retryCount - 1
		retrySecs = retrySecs * 2
	}

	log.Info("Topic dispatch OK.")
}

func dispatchPayload(ctx context.Context, tx *gorm.DB, payload pushPayload, client *messaging.Client) {
	log.Debug("Dispatching...")

	var jsonData bytes.Buffer
	if err := json.NewEncoder(&jsonData).Encode(payload.Events); err != nil {
		log.WithField("error", err).Error("Failed to encode JSON payload for dispatch.")
		sentry.CaptureException(err)
		return
	}

	log.WithField("payload", jsonData.String()).Debug("Dispatching pushes.")

	var ttl = time.Duration(2) * time.Hour
	message := &messaging.MulticastMessage{
		Data: map[string]string{
			"events": jsonData.String(),
		},
		Tokens: payload.RegistrationIds,
		Android: &messaging.AndroidConfig{
			TTL: &ttl,
		},
	}

	// Set payload with exponential backoff
	retryCount := 5
	retrySecs := 10

	var err error
	var response *messaging.BatchResponse

	for {

		if DebugMode {
			response, err = client.SendMulticastDryRun(ctx, message)
		} else {
			response, err = client.SendMulticast(ctx, message)
		}

		GetStatistics().Dispatches++
		if err == nil {
			break
		}

		if retryCount <= 0 {
			break
		}

		log.WithFields(log.Fields{"err": err, "data": json_data.String()}).Error("Failed to send GCM package.")
		GetStatistics().FailedDispatches++
		sentry.CaptureException(err)
		time.Sleep(time.Duration(retrySecs) * time.Second)
		retryCount = retryCount - 1
		retrySecs = retrySecs * 2
	}

	log.WithFields(log.Fields{"success": response.SuccessCount, "failure": response.FailureCount}).Info("Dispatch OK.")
	processResponse(tx, payload.RegistrationIds, response)
}

func processResponse(tx *gorm.DB, registrationIds []string, response *messaging.BatchResponse) {
	if response.FailureCount == 0 {
		return
	}

	for i, singleResponse := range response.Responses {
		if singleResponse.Success {
			continue
		}

		log.WithFields(log.Fields{"error": singleResponse.Error, "msg": singleResponse.MessageID}).Warn("Error while dispatching to token.")
		error := singleResponse.Error
		GetStatistics().FailedMessages++
		if messaging.IsRegistrationTokenNotRegistered(error) {
			log.WithField("apiKey", registrationIds[i]).Info("Removing not registered push key.")
			if err := tx.Where("key = ?", registrationIds[i]).Delete(ApiKey{}).Error; err != nil {
				sentry.CaptureException(err)
			}
		}
	}
}
