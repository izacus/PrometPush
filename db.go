package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	_ "github.com/lib/pq"
)

type Dogodek struct {
	Id              uint64  `json:"id"`
	Y_wgs           float64 `json:"y_wgs"`
	X_wgs           float64 `json:"x_wgs"`
	Kategorija      string  `json:"kategorija"`
	Opis            string  `json:"opis" sql:"type:text"`
	Cesta           string  `json:"cesta"`
	Vzrok           string  `json:"vzrok"`
	OpisEn          string  `json:"opisEn" sql:"type:text"`
	CestaEn         string  `json:"cestaEn"`
	VzrokEn         string  `json:"vzrokEn"`
	Prioriteta      int32   `json:"prioriteta"`
	PrioritetaCeste int32   `json:"prioritetaCeste"`
	Vneseno         uint64  `json:"vneseno"`
	Updated         uint64  `json:"updated"`
	VeljavnostOd    uint64  `json:"veljavnostOd"`
	VeljavnostDo    uint64  `json:"veljavnostDo"`
}

type ApiKey struct {
	Id               int64
	Key              string
	RegistrationTime int64
	UserAgent        string
}

func GetDbConnection() gorm.DB {
	db, err := gorm.Open("postgres", "dbname=promet_push sslmode=disable")
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Failed to connect to database.")
		panic("Could not connect to database!")
	}

	db.LogMode(false)
	db.SingularTable(true)

	if (!db.HasTable(&ApiKey{})) {
		db.AutoMigrate(&Dogodek{}, &ApiKey{})
		db.Model(&ApiKey{}).AddUniqueIndex("idx_api_key", "key")
	}

	return db
}
