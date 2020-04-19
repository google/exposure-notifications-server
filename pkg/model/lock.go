package model

import (
	"time"

	"cloud.google.com/go/datastore"
)

const (
	// LockTable holds locks for long-running batches.
	LockTable = "Lock"
)

// Lock is lock record with an expiration.
type Lock struct {
	Expires time.Time      `datastore:"expires,noindex"`
	K       *datastore.Key `datastore:"__key__"`
}
