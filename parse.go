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
	if err != nil {
		if response != nil {
			log.WithFields(log.Fields{"status": response.Status, "err": err}).Error("Failed to retrieve data from server.")
		} else {
			log.WithFields(log.Fields{"err": err}).Error("Failed to retrieve data from server.")
		}
		return
	}
	defer response.Body.Close()

	dec := json.NewDecoder(response.Body)
	dec.Decode(&data)

	log.WithFields(log.Fields{"status": response.Status, "num": len(data.Dogodki.D)}).Debug("Data retrieval ok.")

	// Save data to database
	db := GetDbConnection()
	defer db.Close()

	var newEventIds []uint64

	tx := db.Begin()
	for _, item := range data.Dogodki.D {
		var count int
		tx.Where("id = ?", item.Id).Model(&Dogodek{}).Count(&count)
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

	log.WithFields(log.Fields{"num": len(data.Dogodki.D), "ids": newEventIds}).Info(len(newEventIds), " new events found.")

	eventIdsChannel <- newEventIds
}
