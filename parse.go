package main

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"net/http"
)

func ParseData(eventIdsChannel chan<- []uint64) {
	var data struct {
		Dogodki struct {
			D []Dogodek `json:"dogodek"`
		} `json:"dogodki"`
	}

	log.Debug("Retrieving traffic data...")
	response, err := http.Get("http://opendata.si/promet/events/")
	defer response.Body.Close()

	if err != nil {
		log.WithFields(log.Fields{"status": response.Status, "err": err}).Error("Failed to retrieve data from server.")
		return
	}

	dec := json.NewDecoder(response.Body)
	dec.Decode(&data)

	log.WithFields(log.Fields{"status": response.Status, "num": len(data.Dogodki.D)}).Debug("Data retrieval ok.")

	// Save data to database
	db := GetDbConnection()
	defer db.Close()

	var newEventIds []uint64

	tx := db.Begin()
	for _, item := range data.Dogodki.D {
		var existing Dogodek
		tx.First(&existing, item.Id)

		// Existing item found
		if existing.Id == item.Id {
			continue
		}

		tx.Create(&item)
		newEventIds = append(newEventIds, item.Id)
	}
	tx.Commit()

	log.WithFields(log.Fields{"num": len(data.Dogodki.D), "ids": newEventIds}).Info(len(newEventIds), " new events found.")

	eventIdsChannel <- newEventIds
}
