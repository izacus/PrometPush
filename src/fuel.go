package main

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/getsentry/raven-go"
	"net/http"
)

type GasStationPrice struct {
	Id      string     `json:"id"`
	Name    string     `json:"name"`
	Address string     `json:"address"`
	X_wgs   float64    `json:"x_wgs"`
	Y_wgs   float64    `json:"y_wgs"`
	Prices  []GasPrice `json:"prices"`
}

type GasPrice struct {
	FuelType string  `json:"type"`
	Price    float64 `json:"price"`
}

// Upstream JSON structure
type JsonGasStationPrice struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	Location struct {
		Coordinates []float64 `json:"coordinates"`
	} `json:"loc"`
	Prices []GasPrice `json:"prices"`
}

func ParseFuelPrices(pricesChannel chan<- []GasStationPrice) error {
	log.Debug("Retrieving gas prices data...")
	url := "https://api.bencinmonitor.si/stations?forMobile=true"
	response, err := http.Get(url)
	if err != nil {
		if response != nil {
			log.WithFields(log.Fields{"status": response.Status, "err": err}).Error("Failed to retrieve data from server.")
		} else {
			log.WithFields(log.Fields{"err": err}).Error("Failed to retrieve data from server.")
		}

		raven.CaptureErrorAndWait(err, nil)
		return err
	}

	dec := json.NewDecoder(response.Body)
	var data struct {
		Stations []JsonGasStationPrice `json:"stations"`
	}

	dec.Decode(&data)
	items := data.Stations

	var prices = make([]GasStationPrice, 0)
	for _, item := range items {
		price := GasStationPrice{
			item.Key,
			item.Name,
			item.Address,
			item.Location.Coordinates[0],
			item.Location.Coordinates[1],
			item.Prices,
		}

		prices = append(prices, price)
	}

	log.WithFields(log.Fields{"status": response.Status, "num": len(items)}).Debug("Gas price retrieval ok.")
	pricesChannel <- prices
	return nil
}
