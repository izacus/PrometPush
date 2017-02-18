package main

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/julienschmidt/httprouter"
	"net/http"
	"time"
)

type JsonEvent struct {
	Id               uint64    `json:"id,string"`
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

var currentData []JsonEvent

func ShowTrafficData(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if currentData == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	enc.Encode(currentData)
}

func ApiService(eventsChannel <-chan []Dogodek, router *httprouter.Router) {
	router.GET("/data", ShowTrafficData)
	log.Info("API hook registered.")

	for {
		events := <-eventsChannel
		var jsonData = make([]JsonEvent, len(events))

		for i, event := range events {
			jsonEvent := JsonEvent{
				event.Id,
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

		currentData = jsonData
		log.WithFields(log.Fields{"data": currentData}).Debug("Updated API data.")
	}
}
