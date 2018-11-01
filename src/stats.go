package main

import (
	"fmt"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/getsentry/raven-go"
	"github.com/julienschmidt/httprouter"
)

type Statistics struct {
	Dispatches       int
	FailedDispatches int
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
	time_now := time.Now()
	time_today := time.Date(time_now.Year(), time_now.Month(), time_now.Day(),
		0, 0, 0, 0, time_now.Location())

	err = tx.Where("vneseno > ?", time_today.Unix()/1000).Model(&Dogodek{}).Count(&count)
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
}

func GetStatistics() *Statistics {
	time_now := time.Now()
	if time_now.Day() != stats.Created.Day() || time_now.Month() != stats.Created.Month() {
		stats = Statistics{}
		stats.Created = time_now
	}

	return &stats
}
