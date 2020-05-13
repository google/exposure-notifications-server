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

// Package cleanup implements the API handlers for running data deletion jobs.
package cleanup

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/serverenv"
)

const (
	ttlEnvVar = "TTL_DURATION"
	minTTL    = 10 * 24 * time.Hour
)

// NewExposureHandler creates a http.Handler for deleting exposure keys
// from the database.
func NewExposureHandler(db *database.DB, config *Config) http.Handler {
	return &exposureCleanupHandler{
		db:     db,
		config: config,
	}
}

type exposureCleanupHandler struct {
	db     *database.DB
	config *Config
}

func (h *exposureCleanupHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	cutoff, err := cutoffDate(h.config.TTL)
	if err != nil {
		logger.Errorf("error processing cutoff time: %v", err)
		http.Error(w, "internal processing error", http.StatusInternalServerError)
		return
	}
	logger.Infof("Starting cleanup for records older than %v", cutoff.UTC())

	// Set timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, h.config.Timeout)
	defer cancel()

	count, err := h.db.DeleteExposures(timeoutCtx, cutoff)
	if err != nil {
		logger.Errorf("Failed deleting exposures: %v", err)
		http.Error(w, "internal processing error", http.StatusInternalServerError)
		return
	}

	logger.Infof("cleanup run complete, deleted %v records.", count)
	w.WriteHeader(http.StatusOK)
}

// NewExportHandler creates a http.Handler that manages deletetion of
// old export files that are no longer needed by clients for download.
func NewExportHandler(db *database.DB, config *Config, env *serverenv.ServerEnv) http.Handler {
	return &exportCleanupHandler{
		db:     db,
		config: config,
		env:    env,
	}
}

type exportCleanupHandler struct {
	db     *database.DB
	config *Config
	env    *serverenv.ServerEnv
}

func (h *exportCleanupHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	cutoff, err := cutoffDate(h.config.TTL)
	if err != nil {
		logger.Errorf("error calculating cutoff time: %v", err)
		http.Error(w, "internal processing error", http.StatusInternalServerError)
		return
	}
	logger.Infof("Starting cleanup for export files older than %v", cutoff.UTC())

	if h.env.Blobstore == nil {
		logger.Errorf("no blob storage system configured")
		http.Error(w, "internal processing error", http.StatusInternalServerError)
		return
	}

	// Set h.Timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, h.config.Timeout)
	defer cancel()

	count, err := h.db.DeleteFilesBefore(timeoutCtx, cutoff, h.env.Blobstore)
	if err != nil {
		logger.Errorf("Failed deleting export files: %v", err)
		http.Error(w, "internal processing error", http.StatusInternalServerError)
		return
	}

	logger.Infof("cleanup run complete, deleted %v files.", count)
	w.WriteHeader(http.StatusOK)
}

func cutoffDate(d time.Duration) (time.Time, error) {
	if d < minTTL {
		return time.Time{}, fmt.Errorf("cleanup ttl is less than configured minumum ttl")
	}
	return time.Now().Add(-d), nil
}
