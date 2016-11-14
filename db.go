package main

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/getsentry/raven-go"
	"github.com/jinzhu/gorm"
	_ "github.com/lib/pq"
)

type Dogodek struct {
	Id              uint64  `json:"Id,string"`
	Y_wgs           float64 `json:"y_wgs"`
	X_wgs           float64 `json:"x_wgs"`
	Kategorija      string  `json:"Kategorija"`
	Opis            string  `json:"Description" sql:"type:text"`
	Cesta           string  `json:"Cesta"`
	Vzrok           string  `json:"Title"`
	OpisEn          string
	CestaEn         string
	VzrokEn         string
	Prioriteta      int32 `json:"Prioriteta"`
	PrioritetaCeste int32 `json:"PrioritetaCeste"`
	MejniPrehod     bool  `json:"isMejniPrehod" sql:"default:false"`
	Vneseno         uint64

	Updated      uint64
	VeljavnostOd uint64
	VeljavnostDo uint64

	UpdatedTime      time.Time `json:"Updated"`
	VeljavnostOdTime time.Time `json:"VeljavnostOd"`
	VeljavnostDoTime time.Time `json:"VeljavnostDo"`
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
		raven.CaptureErrorAndWait(err, nil)
		log.WithFields(log.Fields{"err": err}).Error("Failed to connect to database.")
		panic("Could not connect to database!")
	}

	err = db.DB().Ping()
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		log.WithFields(log.Fields{"err": err}).Error("Failed to connect to database.")
		panic("Could not connect to database!")
	}

	db.LogMode(false)
	db.SingularTable(true)

	if (!db.HasTable(&ApiKey{})) {
		db.AutoMigrate(&ApiKey{})
		db.Model(&ApiKey{}).AddUniqueIndex("idx_api_key", "key")
	}

	db.AutoMigrate(&Dogodek{})
	return db
}
