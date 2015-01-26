package main

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"
	"net/http"
)

func ParseData() []uint64 {
	var data struct {
		Dogodki struct {
			D []Dogodek `json:"dogodek"`
		} `json:"dogodki"`
	}

	log.Info("Retrieving traffic data...")
	response, _ := http.Get("http://opendata.si/promet/events/")
	defer response.Body.Close()

	dec := json.NewDecoder(response.Body)
	dec.Decode(&data)

	log.WithFields(log.Fields{"status": response.Status, "num": len(data.Dogodki.D)}).Info("Data retrieval ok.")

	// Save data to database
	db, _ := gorm.Open("sqlite3", "events.db")
	defer db.Close()

	db.DB()
	db.AutoMigrate(&Dogodek{})

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

	log.WithFields(log.Fields{"ids": newEventIds}).Info(len(newEventIds), " new events found.")

	return newEventIds
}
