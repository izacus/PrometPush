package main

import (
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/getsentry/raven-go"
	"github.com/julienschmidt/httprouter"
	cron "github.com/robfig/cron"
	"github.com/scalingdata/gcfg"
)

type Config struct {
	Push struct {
		ApiKey string
		Dsn    string
	}
}

func main() {
	log.SetLevel(log.InfoLevel)

	// Set logging to file when running in production
	if len(os.Args) > 1 {
		if os.Args[1] == "--production" {
			os.Mkdir("log", 0755)
			f, _ := os.Create("log/promet_push.log")
			defer f.Close()
			log.SetOutput(f)
			log.SetFormatter(new(log.TextFormatter))
		} else if os.Args[1] == "--debug" {
			log.SetLevel(log.DebugLevel)
		}
	}

	configuration := getConfiguration()
	raven.SetDSN(configuration.Push.Dsn)

	eventIdsChannel := make(chan []uint64)
	eventsChannel := make(chan []Dogodek)

	// Dispatcher processor
	router := httprouter.New()

	go PushDispatcher(eventIdsChannel, configuration.Push.ApiKey)
	go ApiService(eventsChannel, router)

	ParseData(eventIdsChannel, eventsChannel)
	c := cron.New()
	c.AddFunc("@every 6m", func() { ParseData(eventIdsChannel, eventsChannel) })
	c.Start()

	// Register HTTP functions
	router.POST("/register", RegisterPush)
	router.POST("/unregister", UnregisterPush)
	router.GET("/stats", ShowStatistics)
	log.Fatal(http.ListenAndServe(":8080", router))
}

func getConfiguration() Config {
	var cfg Config
	err := gcfg.ReadFileInto(&cfg, "promet_push.config")
	if err != nil {
		log.WithField("err", err).Error("Failed to parse configuration.")
	}

	log.WithField("config", cfg).Debug("Read configuration.")
	return cfg
}
