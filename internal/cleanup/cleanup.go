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

	"go.opencensus.io/trace"

	"github.com/google/exposure-notifications-server/internal/export/database"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"

	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/storage"
)

const (
	minTTL = 10 * 24 * time.Hour
)

// NewExposureHandler creates a http.Handler for deleting exposure keys
// from the database.
func NewExposureHandler(config *Config, env *serverenv.ServerEnv) (http.Handler, error) {
	if env.Database() == nil {
		return nil, fmt.Errorf("missing database in server environment")
	}

	return &exposureCleanupHandler{
		config:   config,
		env:      env,
		database: publishdb.New(env.Database()),
	}, nil
}

type exposureCleanupHandler struct {
	config   *Config
	env      *serverenv.ServerEnv
	database *publishdb.PublishDB
}

func (h *exposureCleanupHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "(*cleanup.exposureCleanupHandler).ServeHTTP")
	defer span.End()

	logger := logging.FromContext(ctx)
	metrics := h.env.MetricsExporter(ctx)

	cutoff, err := cutoffDate(h.config.TTL)
	if err != nil {
		message := fmt.Sprintf("error processing cutoff time: %v", err)
		logger.Error(message)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
		metrics.WriteInt("cleanup-exposures-setup-failed", true, 1)
		http.Error(w, "internal processing error", http.StatusInternalServerError)
		return
	}
	logger.Infof("Starting cleanup for records older than %v", cutoff.UTC())
	metrics.WriteInt64("cleanup-exposures-before", false, cutoff.Unix())

	// Set timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, h.config.Timeout)
	defer cancel()

	count, err := h.database.DeleteExposures(timeoutCtx, cutoff)
	if err != nil {
		message := fmt.Sprintf("Failed deleting exposures: %v", err)
		logger.Error(message)
		metrics.WriteInt("cleanup-exposures-delete-failed", true, 1)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
		http.Error(w, "internal processing error", http.StatusInternalServerError)
		return
	}

	metrics.WriteInt64("cleanup-exposures-deleted", true, count)
	logger.Infof("cleanup run complete, deleted %v records.", count)
	w.WriteHeader(http.StatusOK)
}

// NewExportHandler creates a http.Handler that manages deletion of
// old export files that are no longer needed by clients for download.
func NewExportHandler(config *Config, env *serverenv.ServerEnv) (http.Handler, error) {
	if env.Database() == nil {
		return nil, fmt.Errorf("missing database in server environment")
	}
	if env.Blobstore() == nil {
		return nil, fmt.Errorf("missing blobstore in server environment")
	}

	return &exportCleanupHandler{
		config:    config,
		env:       env,
		database:  database.New(env.Database()),
		blobstore: env.Blobstore(),
	}, nil
}

type exportCleanupHandler struct {
	config    *Config
	env       *serverenv.ServerEnv
	database  *database.ExportDB
	blobstore storage.Blobstore
}

func (h *exportCleanupHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "(*cleanup.exportCleanupHandler).ServeHTTP")
	defer span.End()

	logger := logging.FromContext(ctx)
	metrics := h.env.MetricsExporter(ctx)

	cutoff, err := cutoffDate(h.config.TTL)
	if err != nil {
		message := fmt.Sprintf("error calculating cutoff time: %v", err)
		metrics.WriteInt("cleanup-exports-setup-failed", true, 1)
		logger.Error(message)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
		http.Error(w, "internal processing error", http.StatusInternalServerError)
		return
	}
	logger.Infof("Starting cleanup for export files older than %v", cutoff.UTC())
	metrics.WriteInt64("cleanup-exports-before", false, cutoff.Unix())

	// Set h.Timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, h.config.Timeout)
	defer cancel()

	count, err := h.database.DeleteFilesBefore(timeoutCtx, cutoff, h.blobstore)
	if err != nil {
		message := fmt.Sprintf("Failed deleting export files: %v", err)
		logger.Error(message)
		metrics.WriteInt("cleanup-exports-delete-failed", true, 1)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
		http.Error(w, "internal processing error", http.StatusInternalServerError)
		return
	}

	metrics.WriteInt("cleanup-exports-deleted", true, count)
	logger.Infof("cleanup run complete, deleted %v files.", count)
	w.WriteHeader(http.StatusOK)
}

func cutoffDate(d time.Duration) (time.Time, error) {
	if d < minTTL {
		return time.Time{}, fmt.Errorf("cleanup ttl is less than configured minimum ttl")
	}
	return time.Now().Add(-d), nil
}
