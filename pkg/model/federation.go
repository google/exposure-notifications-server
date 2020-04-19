package model

import (
	"time"

	"cloud.google.com/go/datastore"
)

const (
	FederationQueryTable = "FederationQuery"
	FederationSyncTable  = "FederationSync"
)

type FederationQuery struct {
	ServerAddr     string         `datastore:"serverAddr,noindex"`
	IncludeRegions []string       `datastore:"includeRegions,noindex"`
	ExcludeRegions []string       `datastore:"excludeRegions,noindex"`
	LastTimestamp  time.Time      `datastore:"lastTimestamp,noindex"`
	K              *datastore.Key `datastore:"__key__"`
}

type FederationSync struct {
	Started      time.Time      `datastore:"started"`
	Completed    time.Time      `datastore:"completed"`
	Insertions   int            `datastore:"insertions,noindex"`
	MaxTimestamp time.Time      `datastore:"maxTimestamp,noindex"`
	K            *datastore.Key `datastore:"__key__"`
}
