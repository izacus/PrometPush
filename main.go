package main

import (
	"code.google.com/p/gcfg"
	log "github.com/Sirupsen/logrus"
	"github.com/julienschmidt/httprouter"
	cron "github.com/robfig/cron"
	"net/http"
	"os"
	"github.com/getsentry/raven-go"
)

type Config struct {
	Push struct {
		ApiKey string
		Dsn string
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
		}
	}

	configuration := getConfiguration()
	raven.SetDSN(configuration.Push.Dsn)

	eventIdsChannel := make(chan []uint64)

	// Retrieve the starting reference data
	//ParseData(make(chan []uint64, 1))

	// Dispatcher processor
	go PushDispatcher(eventIdsChannel, configuration.Push.ApiKey)

	ParseData(eventIdsChannel)
	c := cron.New()
	c.AddFunc("@every 6m", func() { ParseData(eventIdsChannel) })
	c.Start()

	// Register HTTP functions
	router := httprouter.New()
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
