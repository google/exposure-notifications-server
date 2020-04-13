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
}

// Infection represents the record as storedin the database
type Infection struct {
	DiagnosisKey     []byte         `datastore:"diagnosisKey"`
	AppPackageName   string         `datastore:"appPackageName"`
	Country          string         `datastore:"country"`
	Platform         string         `datastore:"string,noindex"`
	FederationSyncId int64          `datastore:"syncId"`
	BatchTimestamp   time.Time      `datastore:"batchTimestamp"`
	K                *datastore.Key `datstore:"__key__"`
}

func TransformPublish(inData *Publish, batchTime time.Time) ([]Infection, error) {
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
			BatchTimestamp:   batchTime,
		}
		entities = append(entities, infection)
	}
	return entities, nil
}
