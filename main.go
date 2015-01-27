package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/julienschmidt/httprouter"
	cron "github.com/robfig/cron"
	"net/http"
	"os"
)

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

	eventIdsChannel := make(chan []uint64)

	// Retrieve the starting reference data
	ParseData(make(chan []uint64, 1))

	// Dispatcher processor
	go PushDispatcher(eventIdsChannel)

	c := cron.New()
	c.AddFunc("@every 2m", func() { ParseData(eventIdsChannel) })
	c.Start()

	// Register HTTP functions
	router := httprouter.New()
	router.POST("/register", RegisterPush)
	router.DELETE("/register", UnregisterPush)
	log.Fatal(http.ListenAndServe(":8080", router))
}
