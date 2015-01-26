package main

import (
	cron "github.com/robfig/cron"
)

type Dogodek struct {
	Id              uint64  `json:"id"`
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
	Vneseno         uint64  `json:"vneseno"`
	Updated         uint64  `json:"updated"`
	VeljavnostOd    uint64  `json:"veljavnostOd"`
	VeljavnostDo    uint64  `json:"veljavnostDo"`
}

func main() {
	c := cron.New()
	c.AddFunc("@every 10s", func() { ParseData() })
	c.Start()
	select {}
}
