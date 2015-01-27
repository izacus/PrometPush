package main

import (
	log "github.com/Sirupsen/logrus"
)

func PushDispatcher(eventIdsChannel <-chan []uint64) {
	for {
		ids := <-eventIdsChannel
		log.WithField("ids", ids).Debug("New ids received.")
	}
}
