package model

import (
	"encoding/base64"
	"strings"
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
	Region         []string `json:"region"`
	Platform       string   `json:"platform"`
	Verification   string   `json:"verificationPayload"`
	KeyDay         int64    `json:"keyDay"`
}

// Infection represents the record as storedin the database
// TODO(helmick) - refactor this so that there is a public
// Infection struct that doesn't have public fields and an
// internal struct that does. Separate out the database model
// from direct access.append
// Mark records as writable/nowritable - is diagnosis key encrypted
type Infection struct {
	DiagnosisKey     []byte         `datastore:"diagnosisKey,noindex"`
	AppPackageName   string         `datastore:"appPackageName"`
	Region           []string       `datastore:"region"`
	Platform         string         `datastore:"string,noindex"`
	FederationSyncId string         `datastore:"syncId"`
	KeyDay           time.Time      `datastore:"keyDay"`
	CreatedAt        time.Time      `datastore:"createdAt"`
	K                *datastore.Key `datastore:"__key__"`
	// TODO(helmick): Add DiagnosisStatus, VerificationSource
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

// TruncateWindow truncates a time based on the size of the creation window.
func TruncateWindow(t time.Time) time.Time {
	return t.Truncate(createWindow)
}

func TransformPublish(inData *Publish, batchTime time.Time) ([]Infection, error) {
	createdAt := TruncateWindow(batchTime)
	keyDay := truncateDay(inData.KeyDay)
	for i := range inData.Region {
		inData.Region[i] = strings.ToUpper(inData.Region[i])
	}
	entities := make([]Infection, 0, len(inData.Keys))
	for _, diagnosisKey := range inData.Keys {
		binKey, err := base64.StdEncoding.DecodeString(diagnosisKey)
		if err != nil {
			return nil, err
		}
		// TODO - data validation
		// TODO encrypt the diagnosis key (binKey)
		infection := Infection{
			DiagnosisKey:   binKey,
			AppPackageName: inData.AppPackageName,
			Region:         inData.Region,
			Platform:       inData.Platform,
			KeyDay:         keyDay,
			CreatedAt:      createdAt,
		}
		entities = append(entities, infection)
	}
	return entities, nil
}
