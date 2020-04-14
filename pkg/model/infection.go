package model

import (
	"encoding/base64"
	"time"

	"cloud.google.com/go/datastore"
)

const (
	InfectionTable = "infection"
)

// Publish represents the body of the PublishInfectedIds API call.
type Publish struct {
	Keys           []string `json:"diagnosisKeys"`
	AppPackageName string   `json:"appPackageName"`
	Country        string   `json:"country"`
	Platform       string   `json:"platform"`
	Verification   string   `json:"verificationPayload"`
	KeyDay         int64    `json:"keyDay"`
}

// Infection represents the record as storedin the database
type Infection struct {
	DiagnosisKey     []byte         `datastore:"diagnosisKey"`
	AppPackageName   string         `datastore:"appPackageName"`
	Country          string         `datastore:"country"`
	Platform         string         `datastore:"string,noindex"`
	FederationSyncId int64          `datastore:"syncId"`
	KeyDay           time.Time      `datastore:"keyDay"`
	CreatedAt        time.Time      `datastore:"createdAt"`
	K                *datastore.Key `datstore:"__key__"`
}

const (
	oneDay       = time.Hour * 24
	createWindow = time.Minute * 15
)

// Key days are set to midnight (UTC) on the day the key was used.
func truncateDay(utcTimeSec int64) time.Time {
	t := time.Unix(utcTimeSec, 0)
	return t.Truncate(oneDay)
}

// Transforms a time to
func truncateWindow(t time.Time) time.Time {
	return t.Truncate(createWindow)
}

func TransformPublish(inData *Publish, batchTime time.Time) ([]Infection, error) {
	createdAt := truncateWindow(batchTime)
	keyDay := truncateDay(inData.KeyDay)
	entities := make([]Infection, 0, len(inData.Keys))
	for _, diagnosisKey := range inData.Keys {
		binKey, err := base64.StdEncoding.DecodeString(diagnosisKey)
		if err != nil {
			return nil, err
		}
		// TODO - data validation
		// TODO encrypt the diagnosis key (binKey)
		infection := Infection{
			DiagnosisKey:     binKey,
			AppPackageName:   inData.AppPackageName,
			Country:          inData.Country,
			Platform:         inData.Platform,
			FederationSyncId: 0,
			KeyDay:           keyDay,
			CreatedAt:        createdAt,
		}
		entities = append(entities, infection)
	}
	return entities, nil
}
