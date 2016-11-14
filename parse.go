package main

import (
	"encoding/json"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/getsentry/raven-go"
)

func ParseData(eventIdsChannel chan<- []uint64) {
	var data struct {
		Contents []struct {
			Data struct {
				D []Dogodek `json:"Items"`
			} `json:"Data"`
		} `json:"Contents"`
	}

	log.Debug("Retrieving traffic data...")
	response, err := http.Get("http://opendata.si/promet/events/")
	if err != nil {
		if response != nil {
			log.WithFields(log.Fields{"status": response.Status, "err": err}).Error("Failed to retrieve data from server.")
		} else {
			log.WithFields(log.Fields{"err": err}).Error("Failed to retrieve data from server.")
		}

		raven.CaptureErrorAndWait(err, nil)
		return
	}
	defer response.Body.Close()

	dec := json.NewDecoder(response.Body)
	dec.Decode(&data)

	items := data.Contents[0].Data.D
	log.WithFields(log.Fields{"status": response.Status, "num": len(items)}).Debug("Data retrieval ok.")
	log.WithFields(log.Fields{"items": items}).Debug("Items retrieved.")

	// Save data to database
	db := GetDbConnection()
	defer db.Close()

	var newEventIds []uint64

	tx := db.Begin()
	for _, item := range items {
		// Fix up date types
		item.Updated = uint64(item.UpdatedTime.Unix())
		item.VeljavnostDo = uint64(item.VeljavnostDoTime.Unix())
		item.VeljavnostOd = uint64(item.VeljavnostOdTime.Unix())

		var count int
		tx.Where("id = ?", item.Id).Model(&Dogodek{}).Count(&count)
		log.WithFields(log.Fields{"Count": count, "Id": item.Id}).Debug("Checking event.")

		if len(newEventIds) > 0 {
			continue
		}

		if count > 0 {
			continue
		}

		tx.Create(&item)
		newEventIds = append(newEventIds, item.Id)
	}
	tx.Commit()

	log.WithFields(log.Fields{"num": len(items), "ids": newEventIds}).Info(len(newEventIds), " new events found.")

	eventIdsChannel <- newEventIds
}
