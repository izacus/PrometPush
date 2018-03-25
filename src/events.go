package main

import (
	"encoding/json"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/getsentry/raven-go"
)

type Events struct {
	newEventIds []uint64
	events      []Dogodek
}

func getEvents(english bool) ([]Dogodek, error) {
	log.Debug("Retrieving traffic data...")
	url := "https://opendata.si/promet/events/"
	if english {
		url = url + "?lang=en"
	}

	response, err := http.Get(url)
	if err != nil {
		if response != nil {
			log.WithFields(log.Fields{"status": response.Status, "err": err}).Error("Failed to retrieve data from server.")
		} else {
			log.WithFields(log.Fields{"err": err}).Error("Failed to retrieve data from server.")
		}

		raven.CaptureErrorAndWait(err, nil)
		return nil, err
	}

	dec := json.NewDecoder(response.Body)

	var data struct {
		Contents []struct {
			Data struct {
				D []Dogodek `json:"Items"`
			} `json:"Data"`
		} `json:"Contents"`
	}

	dec.Decode(&data)
	if len(data.Contents) == 0 {
		log.WithFields(log.Fields{"contents": data.Contents}).Error("Empty contents retrieved!")
		return nil, err
	}

	items := data.Contents[0].Data.D
	log.WithFields(log.Fields{"status": response.Status, "num": len(items), "english": english}).Debug("Data retrieval ok.")
	return items, nil
}

func ParseTrafficEvents(eventIdsChannel chan<- []uint64, eventsChannel chan<- []Dogodek) {
	items, err := getEvents(false)
	if err != nil {
		return
	}

	itemsEn, err := getEvents(true)
	if err != nil {
		return
	}

	log.WithFields(log.Fields{"items": items, "itemsEn": itemsEn}).Debug("Items retrieved.")

	// Make a map of english events
	itemEnMap := make(map[uint64]Dogodek)
	for _, item := range itemsEn {
		itemEnMap[item.Id] = item
	}

	// Save data to database
	db := GetDbConnection()
	defer db.Close()

	var newEventIds []uint64
	var newItems = make([]Dogodek, 0)

	tx := db.Begin()
	for _, item := range items {
		// Fix up date types
		item.Updated = uint64(item.UpdatedTime.Unix())
		item.VeljavnostDo = uint64(item.VeljavnostDoTime.Unix())
		item.VeljavnostOd = uint64(item.VeljavnostOdTime.Unix())

		// Fill in english data if available
		itemEn, ok := itemEnMap[item.Id]
		if ok {
			item.CestaEn = itemEn.Cesta
			item.OpisEn = itemEn.Opis
			item.VzrokEn = itemEn.Vzrok
		} else {
			log.WithFields(log.Fields{"item": item}).Warn("Couldn't find english item!")
		}

		newItems = append(newItems, item)

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
	eventsChannel <- newItems
}
