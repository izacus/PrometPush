package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/getsentry/raven-go"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

type Statistics struct {
	Dispatches       int
	FailedDispatches int
	FailedMessages   int
	UpdatedPushKeys  int

	DeviceRegistrations          int
	DeviceUnregistrations        int
	DeviceUnregistrationsInvalid int

	Created time.Time
}

var stats Statistics

func ShowStatistics(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	raven.SetHttpContext(raven.NewHttp(r))
	r.Close = true
	db := GetDbConnection()
	tx := db.Begin()
	defer tx.Commit()

	var count int
	err := tx.Model(&ApiKey{}).Count(&count)
	if err.Error != nil {
		log.WithFields(log.Fields{"err": err.Error}).Error("Failed to retrieve statistics.")
		count = -1
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "registered_api_keys:%d\n", count)

	// Find todays events
	timeNow := time.Now()
	timeToday := time.Date(timeNow.Year(), timeNow.Month(), timeNow.Day(),
		0, 0, 0, 0, timeNow.Location())

	err = tx.Where("vneseno > ?", timeToday.Unix()/1000).Model(&Dogodek{}).Count(&count)
	if err.Error != nil {
		log.WithFields(log.Fields{"err": err.Error}).Error("Failed to retrieve statistics.")
		count = -1
	}

	fmt.Fprintf(w, "todays_events:%d\n", count)

	statistics := GetStatistics()
	fmt.Fprintf(w, "today_dispatches:%d\n", statistics.Dispatches)
	fmt.Fprintf(w, "today_failed_dispatches:%d\n", statistics.FailedDispatches)
	fmt.Fprintf(w, "today_device_registrations:%d\n", statistics.DeviceRegistrations)
	fmt.Fprintf(w, "today_device_unregistrations:%d\n", statistics.DeviceUnregistrations)
	fmt.Fprintf(w, "today_device_unregistrations_invalid:%d\n", statistics.DeviceUnregistrationsInvalid)
	fmt.Fprintf(w, "today_device_updatedkeys:%d\n", statistics.UpdatedPushKeys)
	fmt.Fprintf(w, "today_failed_messages:%d\n", statistics.FailedMessages)
}

func GetStatistics() *Statistics {
	timeNow := time.Now()
	if timeNow.Day() != stats.Created.Day() || timeNow.Month() != stats.Created.Month() {
		stats = Statistics{}
		stats.Created = timeNow
	}

	return &stats
}
