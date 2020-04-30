// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/googlepartners/exposure-notifications/internal/database"
	"github.com/googlepartners/exposure-notifications/internal/logging"
)

const (
	ttlEnvVar         = "TTL_DURATION"
	minCutoffDuration = "10d"
)

func NewInfectionWipeoutHandler(db *database.DB, timeout time.Duration) http.Handler {
	return &infectionWipeoutHandler{
		db:      db,
		timeout: timeout,
	}
}

type infectionWipeoutHandler struct {
	db      *database.DB
	timeout time.Duration
}

func (h *infectionWipeoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	cutoff, err := getCutoff(ttlEnvVar)
	if err != nil {
		logger.Errorf("error getting cutoff time: %v", err)
		http.Error(w, "internal processing error", http.StatusInternalServerError)
		return
	}
	logger.Infof("Starting wipeout for records older than %v", cutoff.UTC())

	// Set h.Timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	count, err := h.db.DeleteInfections(timeoutCtx, cutoff)
	if err != nil {
		logger.Errorf("Failed deleting infections: %v", err)
		http.Error(w, "internal processing error", http.StatusInternalServerError)
		return
	}

	logger.Infof("wipeout run complete, deleted %v records.", count)
	w.WriteHeader(http.StatusOK)
}

func NewExportWipeoutHandler(db *database.DB, timeout time.Duration) http.Handler {
	return &exportWipeoutHandler{
		db:      db,
		timeout: timeout,
	}
}

type exportWipeoutHandler struct {
	db      *database.DB
	timeout time.Duration
}

func (h *exportWipeoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	cutoff, err := getCutoff(ttlEnvVar)
	if err != nil {
		logger.Errorf("error getting cutoff time: %v", err)
		http.Error(w, "internal processing error", http.StatusInternalServerError)
		return
	}
	logger.Infof("Starting wipeout for export files older than %v", cutoff.UTC())

	// Set h.Timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	count, err := h.db.DeleteInfections(timeoutCtx, cutoff)
	if err != nil {
		logger.Errorf("Failed deleting export files: %v", err)
		http.Error(w, "internal processing error", http.StatusInternalServerError)
		return
	}

	logger.Infof("wipeout run complete, deleted %v files.", count)
	w.WriteHeader(http.StatusOK)
}

func getCutoff(ttlVar string) (cutoff time.Time, err error) {
	// Parse and Validate TTL duration string.
	ttlString := os.Getenv(ttlVar)
	ttlDuration, err := getAndValidateDuration(ttlString)
	if err != nil {
		err = fmt.Errorf("TTL env variable error: %v", err)
		return cutoff, err
	}

	// Parse and Validate min ttl duration string.
	minTtl, err := getAndValidateDuration(minCutoffDuration)
	if err != nil {
		err = fmt.Errorf("min ttl const error: %v", err)
		return cutoff, err
	}

	// Validate that TTL is sufficiently in the past.
	if ttlDuration < minTtl {
		err = fmt.Errorf("wipeout ttl is less than configured minumum ttl")
		return cutoff, err
	}

	// Get cutoff timestamp
	cutoff = time.Now().UTC().Add(-ttlDuration)
	return cutoff, nil
}

func getAndValidateDuration(durationString string) (time.Duration, error) {
	if durationString == "" {
		return 0, errors.New("not set")
	}
	duration, err := time.ParseDuration(durationString)
	if err != nil {
		return 0, err
	}
	return duration, nil
}
