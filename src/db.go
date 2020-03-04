package main

import (
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	gomigrate "gopkg.in/gormigrate.v1"
)

type Dogodek struct {
	Id              string  `json:"Id"`
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

var db *gorm.DB

func InitializeDbConnection() error {
	var err error

	if DebugMode {
		db, err = gorm.Open("sqlite3", "debug.db")
	} else {
		db, err = gorm.Open("postgres", "dbname=promet_push sslmode=disable")
	}

	if err != nil {
		sentry.CaptureException(err)
		log.WithFields(log.Fields{"err": err}).Error("Failed to connect to database.")
		panic("Could not connect to database!")
	}

	err = db.DB().Ping()
	if err != nil {
		sentry.CaptureException(err)
		log.WithFields(log.Fields{"err": err}).Error("Failed to connect to database.")
		panic("Could not connect to database!")
	}

	db.DB().SetMaxIdleConns(10)

	db.LogMode(DebugMode)
	db.SingularTable(true)

	if (!db.HasTable(&ApiKey{})) {
		db.AutoMigrate(&ApiKey{})
		db.Model(&ApiKey{}).AddUniqueIndex("idx_api_key", "key")
	}

	result := db.AutoMigrate(&Dogodek{})
	if result.Error != nil {
		raven.CaptureErrorAndWait(result.Error, nil)
		log.WithFields(log.Fields{"err": err}).Error("Failed to migrate database!")
		return result.Error
	}

	// We don't run migrations on SQLite3
	if DebugMode {
		return nil
	}

	migration := gomigrate.New(db, gomigrate.DefaultOptions, []*gomigrate.Migration{
		{
			ID: "201803251900",
			Migrate: func(tx *gorm.DB) error {
				return tx.Table("dogodek").ModifyColumn("id", "text").AddIndex("idx_event_id", "id").Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.RemoveIndex("idx_event_id").Table("dogodek").ModifyColumn("id", "bigint").Error
			},
		},
	})

	if err = migration.Migrate(); err != nil {
		log.Fatalf("Could not migrate: %v", err)
		sentry.CaptureException(err)
	}

	return nil
}

func GetDbConnection() *gorm.DB {
	return db
}
