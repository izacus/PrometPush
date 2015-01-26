package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Dogodek struct {
	Id              int64   `json:"id"`
	Y_wgs           float64 `json:"y_wgs"`
	X_wgs           float64 `json:"x_wgs"`
	Kategorija      string  `json:"kategorija"`
	Opis            string  `json:"opis"`
	Cesta           string  `json:"cesta"`
	Vzrok           string  `json:"vzrok"`
	OpisEn          string  `json:"opisEn"`
	CestaEn         string  `json:"cestaEn"`
	VzrokEn         string  `json:"vzrokEn"`
	Prioriteta      int32   `json:"prioriteta"`
	PrioritetaCeste int32   `json:"prioritetaCeste"`
	Vneseno         int64   `json:"vneseno"`
	Updated         int64   `json:"updated"`
	VeljavnostOd    int64   `json:"veljavnostOd"`
	VeljavnostDo    int64   `json:"veljavnostDo"`
}

func main() {
	var data struct {
		Dogodki struct {
			D []Dogodek `json:"dogodek"`
		} `json:"dogodki"`
	}

	response, _ := http.Get("http://opendata.si/promet/events/")
	defer response.Body.Close()

	dec := json.NewDecoder(response.Body)
	dec.Decode(&data)

	for _, item := range data.Dogodki.D {
		fmt.Printf("%d - %s/%s\n", item.Id, item.Cesta, item.Vzrok)
	}
}
