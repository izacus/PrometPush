package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/julienschmidt/httprouter"
	cron "github.com/robfig/cron"
	"net/http"
)

func main() {
	log.SetLevel(log.InfoLevel)

	ParseData()

	c := cron.New()
	c.AddFunc("@every 2m", func() { ParseData() })
	c.Start()

	// Register HTTP functions
	router := httprouter.New()
	router.POST("/register", RegisterPush)
	log.Fatal(http.ListenAndServe(":8080", router))
}
