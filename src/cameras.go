package src

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/getsentry/sentry-go"
	log "github.com/sirupsen/logrus"
)

type Camera struct {
	LocationId string  `json:"location_id"`
	Region     string  `json:"region"`
	Text       string  `json:"text"`
	ImageURL   string  `json:"image_url"`
	X_wgs      float64 `json:"x_wgs"`
	Y_wgs      float64 `json:"y_wgs"`
}

type JsonCamera struct {
	Region string `json:"Region"`
	Image  string `json:"Image"`
	Text   string `json:"Text"`
}

type JsonLocation struct {
	Id          string       `json:"Id"`
	Title       string       `json:"Title"`
	Description string       `json:"Description"`
	X_wgs       float64      `json:"x_wgs"`
	Y_wgs       float64      `json:"y_wgs"`
	Cameras     []JsonCamera `json:"Kamere"`
}

func ParseTrafficCameras(camerasChannel chan<- []Camera) error {
	log.Debug("Retrieving camera data...")
	url := "https://opendata.si/promet/cameras/"
	response, err := http.Get(url)
	if err != nil {
		if response != nil {
			log.WithFields(log.Fields{"status": response.Status, "err": err}).Error("Failed to retrieve data from server.")
		} else {
			log.WithFields(log.Fields{"err": err}).Error("Failed to retrieve data from server.")
		}

		sentry.CaptureException(err)
		return err
	}

	dec := json.NewDecoder(response.Body)
	var data struct {
		Contents []struct {
			Data struct {
				C []JsonLocation `json:"Items"`
			} `json:"Data"`
		} `json:"Contents"`
	}

	decodeErr := dec.Decode(&data)
	if decodeErr != nil {
		buf := new(bytes.Buffer)
		buf.ReadFrom(response.Body)

		sentry.AddBreadcrumb(&sentry.Breadcrumb{
			Category: "upstream-api",
			Message:  buf.String(),
			Level:    "error",
		})

		sentry.CaptureException(decodeErr)
		if decodeErr != nil {
			sentry.CaptureException(decodeErr)
		} else {
			sentry.CaptureMessage("Invalid upstream server response!")
		}

		log.Error("Invalid response from server!")
		return err
	}

	if len(data.Contents) == 0 {
		sentry.CaptureMessage("No camera data retrieved.")
		return nil
	}

	items := data.Contents[0].Data.C

	var cameras = make([]Camera, 0)
	for _, item := range items {
		for _, jsonCamera := range item.Cameras {
			camera := Camera{
				item.Id,
				jsonCamera.Region,
				jsonCamera.Text,
				jsonCamera.Image,
				item.X_wgs,
				item.Y_wgs,
			}

			cameras = append(cameras, camera)
		}
	}

	log.WithFields(log.Fields{"status": response.Status, "num": len(items)}).Debug("Camera retrieval ok.")
	camerasChannel <- cameras
	return nil
}
