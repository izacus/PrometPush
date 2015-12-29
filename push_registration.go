package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/julienschmidt/httprouter"
	"io/ioutil"
	"net/http"
	"time"
	"github.com/getsentry/raven-go"
)

func RegisterPush(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	raven.SetHttpContext(raven.NewHttp(r))
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Failed to read ApiKey from request.")
		raven.CaptureErrorAndWait(err, nil)
		return
	}

	apiKeyStr := string(b)

	// Check if key already exists
	db := GetDbConnection()
	defer db.Close()

	tx := db.Begin()
	// Check for existing registration
	var count int32
	query := tx.Model(&ApiKey{}).Where("key = ?", apiKeyStr).Count(&count)
	if query.Error != nil {
		raven.CaptureErrorAndWait(query.Error, nil)
		log.WithFields(log.Fields{"err": query.Error}).Error("Failed to save new apikey to DB.")
		returnError(w)
		tx.Rollback()
		return
	}
	if count == 0 {
		query = tx.Create(ApiKey{Key: apiKeyStr, RegistrationTime: time.Now().Unix(), UserAgent: r.UserAgent()})
		if query.Error != nil {
			raven.CaptureErrorAndWait(query.Error, nil)
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

func UnregisterPush(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	raven.SetHttpContext(raven.NewHttp(r))
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Failed to read ApiKey from request.")
		return
	}

	apiKeyStr := string(b)

	// Check if key already exists
	db := GetDbConnection()
	defer db.Close()
	query := db.Where("key = ?", apiKeyStr).Delete(ApiKey{})
	if query.Error != nil {
		raven.CaptureErrorAndWait(query.Error, nil)
		log.WithFields(log.Fields{"err": query.Error, "apiKey": apiKeyStr, "ua": r.UserAgent()}).Error("Failed to unregister api api key!")
		returnError(w)
		return
	}

	log.WithFields(log.Fields{"apiKey": apiKeyStr, "ua": r.UserAgent()}).Info("Removed API key registration.")
	GetStatistics().DeviceUnregistrations++

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func returnError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("Failed to process request."))
}
