package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	"github.com/julienschmidt/httprouter"
	_ "github.com/mattn/go-sqlite3"
	"io/ioutil"
	"net/http"
	"time"
)

type ApiKey struct {
	Id               int64
	Key              string
	RegistrationTime int64 `sql:"DEFAULT:current_timestamp"`
	UserAgent        string
}

func RegisterPush(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Failed to read ApiKey from request.")
		return
	}

	apiKeyStr := string(b)
	log.WithFields(log.Fields{"body": apiKeyStr}).Debug("ApiKey for registration.")

	// Check if key already exists
	db, _ := gorm.Open("sqlite3", "events.db")
	defer db.Close()

	db.DB()
	db.LogMode(true)
	query := db.AutoMigrate(&ApiKey{})
	db.Model(&ApiKey{}).AddUniqueIndex("idx_api_key", "key")

	if query.Error != nil {
		log.WithFields(log.Fields{"err": query.Error}).Error("Failed to save new apikey to DB.")
		returnError(w)
		return
	}

	// Check for existing registration
	var count int32
	query = db.Model(&ApiKey{}).Where("key = ?", apiKeyStr).Count(&count)
	if query.Error != nil {
		log.WithFields(log.Fields{"err": query.Error}).Error("Failed to save new apikey to DB.")
		returnError(w)
		return
	}

	if count == 0 {
		query = db.Create(ApiKey{Key: apiKeyStr, RegistrationTime: time.Now().Unix(), UserAgent: r.UserAgent()})
		if query.Error != nil {
			log.WithFields(log.Fields{"err": query.Error}).Error("Failed to save new apikey to DB.")
			db.Rollback()
			returnError(w)
			return
		}
	}

	log.WithFields(log.Fields{"apiKey": apiKeyStr, "ua": r.UserAgent()}).Info("New API key registered.")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func returnError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("Failed to register ApiKey."))
}
