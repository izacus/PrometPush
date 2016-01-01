package main

import (
    "fmt"
    "time"
    "net/http"
    "github.com/julienschmidt/httprouter"
    log "github.com/Sirupsen/logrus"
    "github.com/getsentry/raven-go"
)

type Statistics struct {
    Dispatches int
    FailedDispatches int
    UpdatedPushKeys int

    DeviceRegistrations int
    DeviceUnregistrations int

    Created time.Time
}

var stats Statistics

func ShowStatistics(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
    raven.SetHttpContext(raven.NewHttp(r))
    db := GetDbConnection()
    defer db.Close()

    var count int 
    err := db.Model(&ApiKey{}).Count(&count)
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

    err = db.Where("vneseno > ?", time_today.Unix() / 1000).Model(&Dogodek{}).Count(&count)
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