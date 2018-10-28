package main

import (
	"bytes"
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

	decode_error := dec.Decode(&data)
	if decode_error != nil || len(data.Contents) == 0 {
		buf := new(bytes.Buffer)
		buf.ReadFrom(response.Body)

		if decode_error != nil {
			raven.CaptureErrorAndWait(decode_error, map[string]string{"response": buf.String()})
		} else {
			raven.CaptureMessageAndWait("Invalid response received!", map[string]string{"response": buf.String()})
		}

		log.Error("Invalid response from server!")
		return nil, err
	}

	items := data.Contents[0].Data.D
	log.WithFields(log.Fields{"status": response.Status, "num": len(items), "english": english}).Debug("Data retrieval ok.")
	return items, nil
}

func ParseTrafficEvents(eventIdsChannel chan<- []string, eventsChannel chan<- []Dogodek) {
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
	itemEnMap := make(map[string]Dogodek)
	for _, item := range itemsEn {
		itemEnMap[item.Id] = item
	}

	// Save data to database
	db := GetDbConnection()
	defer db.Close()

	var newEventIds []string
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
		if err := tx.Where("id = ?", item.Id).Model(&Dogodek{}).Count(&count).Error; err != nil {
			raven.CaptureErrorAndWait(err, nil)
			continue
		}

		log.WithFields(log.Fields{"Count": count, "Id": item.Id}).Debug("Checking event.")

		if count > 0 {
			continue
		}

		result := tx.Create(&item)
		if result.Error != nil {
			log.WithFields(log.Fields{"err": result.Error}).Error("Failed to create item!")
			raven.CaptureErrorAndWait(result.Error, nil)
		}

		newEventIds = append(newEventIds, item.Id)
	}

	result := tx.Commit()
	if result.Error != nil {
		raven.CaptureErrorAndWait(result.Error, nil)
	}

	log.WithFields(log.Fields{"num": len(items), "ids": newEventIds}).Info(len(newEventIds), " new events found.")
	eventIdsChannel <- newEventIds
	eventsChannel <- newItems
}
