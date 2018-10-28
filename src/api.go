package main

import (
	"encoding/json"
	"hash/fnv"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/julienschmidt/httprouter"
)

type JsonEvent struct {
	Id               int64     `json:"id,string"`
	Y_wgs            float64   `json:"y_wgs"`
	X_wgs            float64   `json:"x_wgs"`
	Category         string    `json:"category"`
	DescriptionSl    string    `json:"description_sl"`
	DescriptionEn    string    `json:"description_en"`
	RoadSl           string    `json:"road_sl"`
	RoadEn           string    `json:"road_en"`
	CauseSl          string    `json:"cause_sl"`
	CauseEn          string    `json:"cause_en"`
	Priority         int32     `json:"priority"`
	RoadPriority     int32     `json:"road_priority"`
	IsBorderCrossing bool      `json:"is_border_crossing"`
	Updated          time.Time `json:"updated"`
	ValidFrom        time.Time `json:"valid_from"`
	ValidTo          time.Time `json:"valid_to"`
}

type ApiResponse struct {
	Events  []JsonEvent       `json:"events"`
	Cameras []Camera          `json:"cameras"`
	Prices  []GasStationPrice `json:"prices"`
}

var currentEvents []JsonEvent
var currentCameras []Camera
var currentPrices []GasStationPrice

func ShowTrafficData(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	r.Close = true
	if currentEvents == nil || currentCameras == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)

	var events []JsonEvent
	var cameras []Camera
	var prices []GasStationPrice

	if currentEvents == nil {
		events = make([]JsonEvent, 0)
	} else {
		events = currentEvents
	}

	if currentCameras == nil {
		cameras = make([]Camera, 0)
	} else {
		cameras = currentCameras
	}

	if currentPrices == nil {
		prices = make([]GasStationPrice, 0)
	} else {
		prices = currentPrices
	}

	enc.Encode(ApiResponse{events, cameras, prices})
}

func eventService(eventsChannel <-chan []Dogodek) {
	for {
		events := <-eventsChannel
		var jsonData = make([]JsonEvent, len(events))

		for i, event := range events {
			// Calculate Id hash
			algo := fnv.New32a()
			algo.Write([]byte(event.Id))
			id_hash := int64(algo.Sum32())

			jsonEvent := JsonEvent{
				id_hash,
				event.Y_wgs,
				event.X_wgs,
				event.Kategorija,
				event.Opis,
				event.OpisEn,
				event.Cesta,
				event.CestaEn,
				event.Vzrok,
				event.VzrokEn,
				event.Prioriteta,
				event.PrioritetaCeste,
				event.MejniPrehod,
				event.UpdatedTime,
				event.VeljavnostOdTime,
				event.VeljavnostDoTime,
			}

			jsonData[i] = jsonEvent
		}

		currentEvents = jsonData
		log.WithFields(log.Fields{"data": currentEvents}).Debug("Updated event data.")
	}
}

func cameraService(camerasChannel <-chan []Camera) {
	for {
		cameras := <-camerasChannel
		currentCameras = cameras
		log.WithFields(log.Fields{"data": currentCameras}).Debug("Updated camera data.")
	}
}

func gasPricesService(pricesChannel <-chan []GasStationPrice) {
	for {
		prices := <-pricesChannel
		currentPrices = prices
		log.WithFields(log.Fields{"data": currentCameras}).Debug("Updated price data.")
	}
}

func ApiService(eventsChannel <-chan []Dogodek,
	camerasChannel <-chan []Camera,
	pricesChannel <-chan []GasStationPrice,
	router *httprouter.Router) {
	router.GET("/data", ShowTrafficData)
	log.Info("API hook registered.")
	go eventService(eventsChannel)
	go cameraService(camerasChannel)
	go gasPricesService(pricesChannel)
}
