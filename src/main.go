package main

import (
	"net/http"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/julienschmidt/httprouter"
	"github.com/robfig/cron"
	"github.com/scalingdata/gcfg"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	Push struct {
		Dsn            string
		FirebaseJson   string
		IndividualPush bool
	}
}

var GitCommit string
var BuildDate string
var DebugMode bool

func main() {
	DebugMode = false
	log.SetLevel(log.InfoLevel)

	// Set logging to file when running in production
	if len(os.Args) > 1 {
		if os.Args[1] == "--production" {
			os.Mkdir("log", 0755)
			f, err := os.OpenFile("log/promet_push.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				sentry.CaptureException(err)
				log.WithField("error", err).Error("Failed to open log file")
			}
			defer f.Close()
			log.SetOutput(f)
			log.SetFormatter(new(log.TextFormatter))
		} else if os.Args[1] == "--debug" {
			DebugMode = true
			log.SetLevel(log.DebugLevel)
		}
	}
	if len(GitCommit) == 0 {
		GitCommit = "UNKNOWN"
	}
	if len(BuildDate) == 0 {
		BuildDate = "UNKNOWN"
	}

	log.Infof("PrometPush version %s built on %s", GitCommit, BuildDate)

	configuration := getConfiguration()
	if !DebugMode {
		// Since this is a daemon we want to use sync transport
		// to Sentry server to make sure errors always reach it.
		transport := sentry.NewHTTPSyncTransport()
		transport.Timeout = time.Second * 3

		clientOptions := sentry.ClientOptions{
			Dsn:       configuration.Push.Dsn,
			Transport: transport,
		}

		if GitCommit != "UNKNOWN" {
			clientOptions.Release = GitCommit
		}

		err := sentry.Init(clientOptions)
		if err != nil {
			log.WithField("error", err).Error("Failed to initialize Sentry!")
		}
	}

	if err := InitializeDbConnection(); err != nil {
		sentry.CaptureException(err)
		panic("Failed to connect to database")
	}

	db := GetDbConnection()
	defer db.Close()

	eventIdsChannel := make(chan []string)
	eventsChannel := make(chan []Dogodek)
	camerasChannel := make(chan []Camera)
	pricesChannel := make(chan []GasStationPrice)

	// Dispatcher processor
	router := httprouter.New()

	go PushDispatcher(eventIdsChannel, configuration.Push.FirebaseJson)
	go ApiService(eventsChannel, camerasChannel, pricesChannel, router)

	ParseTrafficEvents(eventIdsChannel, eventsChannel)
	ParseTrafficCameras(camerasChannel)
	// Fuel prices are disabled because they're broken.
	//ParseFuelPrices(pricesChannel)
	c := cron.New()
	c.AddFunc("@every 6m", func() { ParseTrafficEvents(eventIdsChannel, eventsChannel) })
	//c.AddFunc("@every 6m", func() { ParseFuelPrices(pricesChannel) })
	c.AddFunc("@every 30m", func() { ParseTrafficCameras(camerasChannel) })
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
