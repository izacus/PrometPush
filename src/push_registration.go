package src

import (
	"io/ioutil"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

// RegisterPush registers a new push target device.
func RegisterPush(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetContext("Request", map[string]string{
			"Method": "GET",
			"URL":    "/register",
		})
	})

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Failed to read ApiKey from request.")
		sentry.CaptureException(err)
		return
	}

	apiKeyStr := string(b)

	// Check if key already exists
	db := GetDbConnection()

	tx := db.Begin()
	// Check for existing registration
	var count int32
	query := tx.Model(&ApiKey{}).Where("key = ?", apiKeyStr).Count(&count)
	if query.Error != nil {
		sentry.CaptureException(query.Error)
		log.WithFields(log.Fields{"err": query.Error}).Error("Failed to save new apikey to DB.")
		returnError(w)
		tx.Rollback()
		return
	}
	if count == 0 {
		query = tx.Create(&ApiKey{Key: apiKeyStr, RegistrationTime: time.Now().Unix(), UserAgent: r.UserAgent()})
		if query.Error != nil {
			sentry.CaptureException(query.Error)
			log.WithFields(log.Fields{"err": query.Error}).Error("Failed to save new apikey to DB.")
			tx.Rollback()
			returnError(w)
			return
		}

		log.WithFields(log.Fields{"apiKey": apiKeyStr, "ua": r.UserAgent()}).Info("New API key registered.")
		GetStatistics().DeviceRegistrations++
	} else {
		log.WithFields(log.Fields{"apiKey": apiKeyStr, "ua": r.UserAgent()}).Info("Skipping existing API key.")
	}

	tx.Commit()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// UnregisterPush removes registration of a client.
func UnregisterPush(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetContext("Request", map[string]string{
			"Method": "GET",
			"URL":    "/unregister",
		})
	})

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Failed to read ApiKey from request.")
		return
	}

	apiKeyStr := string(b)

	// Check if key already exists
	db := GetDbConnection()

	tx := db.Begin()
	var count int
	if err := tx.Where("key = ?", apiKeyStr).Model(&ApiKey{}).Count(&count).Error; err != nil {
		sentry.CaptureException(err)
		log.WithFields(log.Fields{"err": err, "apiKey": apiKeyStr, "ua": r.UserAgent()}).Error("Failed to check for API key!")
		tx.Rollback()
		return
	}

	if count > 0 {
		query := tx.Where("key = ?", apiKeyStr).Delete(ApiKey{})
		if query.Error != nil {
			sentry.CaptureException(query.Error)
			log.WithFields(log.Fields{"err": query.Error, "apiKey": apiKeyStr, "ua": r.UserAgent()}).Error("Failed to unregister api api key!")
			returnError(w)
			tx.Rollback()
			return
		}

		log.WithFields(log.Fields{"apiKey": apiKeyStr, "ua": r.UserAgent()}).Info("Removed API key registration.")
		GetStatistics().DeviceUnregistrations++
	} else {
		log.WithFields(log.Fields{"apiKey": apiKeyStr, "ua": r.UserAgent()}).Info("API key for removal not found.")
		GetStatistics().DeviceUnregistrationsInvalid++
	}

	tx.Commit()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func returnError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("Failed to process request."))
}
