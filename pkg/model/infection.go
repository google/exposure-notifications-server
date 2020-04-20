package model

import (
	"encoding/base64"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
)

const (
	// InfectionTable holds uploaded infected keys.
	InfectionTable = "infection"
)

// Publish represents the body of the PublishInfectedIds API call.
type Publish struct {
	Keys            []DiagnosisKey `json:"diagnosisKeys"`
	Regions         []string       `json:"regions"`
	AppPackageName  string         `json:"appPackageName"`
	DiagnosisStatus int            `json:"diagnosisStatus"`
	Verification    string         `json:"verificationPayload"`
}

// DiagnosisKey is the 16 byte key, the start time of the key and the
// duration of the key. A duration of 0 means 24 hours.
type DiagnosisKey struct {
	Key         string `json:"key"`
	KeyDay      int64  `json:"keyDay"`
	KeyDuration int64  `json:"keyDuration"`
}

// Infection represents the record as storedin the database
// TODO(helmick) - refactor this so that there is a public
// Infection struct that doesn't have public fields and an
// internal struct that does. Separate out the database model
// from direct access.
// Mark records as writable/nowritable - is diagnosis key encrypted
type Infection struct {
	DiagnosisKey    []byte         `datastore:"diagnosisKey,noindex"`
	DiagnosisStatus int            `datastore:"diagnosisStatus,noindex"`
	AppPackageName  string         `datastore:"appPackageName,noindex"`
	Regions         []string       `datastore:"region,noindex"`
	FederationSync  *datastore.Key `datastore:"sync,noindex"`
	KeyDay          time.Time      `datastore:"keyDay,noindex"`
	KeyDuration     int64          `datastore:"keyDuration,noindex"`
	CreatedAt       time.Time      `datastore:"createdAt"`
	LocalProvenance bool           `datastore:"localProvenance"`
	K               *datastore.Key `datastore:"__key__"`
	// TODO(helmick): Add DiagnosisStatus, VerificationSource
}

const (
	oneDay       = time.Hour * 24
	createWindow = time.Minute * 15
)

// TruncateDay truncates the given timestamp to midnight (UTC).
func TruncateDay(utcTimeSec int64) time.Time {
	t := time.Unix(utcTimeSec, 0)
	return t.Truncate(oneDay)
}

// TruncateWindow truncates a time based on the size of the creation window.
func TruncateWindow(t time.Time) time.Time {
	return t.Truncate(createWindow)
}

// TransformPublish converts incoming key data to a list of infection entities.
func TransformPublish(inData *Publish, batchTime time.Time) ([]Infection, error) {
	createdAt := TruncateWindow(batchTime)
	entities := make([]Infection, 0, len(inData.Keys))

	// Regions are a multi-value property, uppercase them for storage.
	upcaseRegions := make([]string, len(inData.Regions))
	for i, r := range inData.Regions {
		upcaseRegions[i] = strings.ToUpper(r)
	}

	for _, diagnosisKey := range inData.Keys {
		binKey, err := base64.StdEncoding.DecodeString(diagnosisKey.Key)
		if err != nil {
			return nil, err
		}
		keyDay := TruncateDay(diagnosisKey.KeyDay)
		// TODO(helmick) - data validation
		infection := Infection{
			DiagnosisKey:    binKey,
			DiagnosisStatus: inData.DiagnosisStatus,
			AppPackageName:  inData.AppPackageName,
			Regions:         upcaseRegions,
			KeyDay:          keyDay,
			KeyDuration:     diagnosisKey.KeyDuration,
			CreatedAt:       createdAt,
			LocalProvenance: true, // This is the origin system for this data.
		}
		entities = append(entities, infection)
	}
	return entities, nil
}
