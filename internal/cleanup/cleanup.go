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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.opencensus.io/stats"

	"github.com/google/exposure-notifications-server/internal/export/database"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"
	"github.com/hashicorp/go-multierror"

	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/google/exposure-notifications-server/pkg/logging"
)

const minTTL = 10 * 24 * time.Hour

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
	ctx := r.Context()
	logger := logging.FromContext(ctx).Named("cleanup.exposure")

	cutoff, err := cutoffDate(h.config.TTL, h.config.DebugOverrideCleanupMinDuration)
	if err != nil {
		logger.Errorw("failed to calculate cutoff date", "error", err)
		respondWithError(w, http.StatusInternalServerError, err)
		return
	}

	// Construct a multi-error. If one of the purges fails, we still want to
	// attempt the other purges.
	var merr *multierror.Error

	// Exposures
	func() {
		ctx, cancel := context.WithTimeout(ctx, h.config.Timeout)
		defer cancel()

		if count, err := h.database.DeleteExposuresBefore(ctx, cutoff); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("failed to delete exposures: %w", err))
		} else {
			logger.Infow("purged exposures", "count", count)
		}
	}()

	// Stats
	func() {
		ctx, cancel := context.WithTimeout(ctx, h.config.Timeout)
		defer cancel()

		if count, err := h.database.DeleteStatsBefore(ctx, cutoff); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("failed to delete stats: %w", err))
		} else {
			logger.Infow("purged statistics", "count", count)
		}
	}()

	if merr != nil {
		logger.Errorw("failed to cleanup exposures", "errors", merr.WrappedErrors())
		respondWithError(w, http.StatusInternalServerError, merr)
		return
	}

	stats.Record(ctx, mExposureSuccess.M(1))
	respond(w, http.StatusOK)
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
	ctx := r.Context()
	logger := logging.FromContext(ctx).Named("cleanup.export")

	cutoff, err := cutoffDate(h.config.TTL, h.config.DebugOverrideCleanupMinDuration)
	if err != nil {
		logger.Errorw("failed to calculate cutoff date", "error", err)
		respondWithError(w, http.StatusInternalServerError, err)
		return
	}

	// Construct a multi-error. If one of the purges fails, we still want to
	// attempt the other purges.
	var merr *multierror.Error

	// Files
	func() {
		ctx, cancel := context.WithTimeout(ctx, h.config.Timeout)
		defer cancel()

		if count, err := h.database.DeleteFilesBefore(ctx, cutoff, h.blobstore); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("failed to delete files: %w", err))
		} else {
			logger.Infow("purged files", "count", count)
		}
	}()

	if merr != nil {
		logger.Errorw("failed to cleanup exports", "errors", merr.WrappedErrors())
		respondWithError(w, http.StatusInternalServerError, merr)
		return
	}

	stats.Record(ctx, mExportSuccess.M(1))
	respond(w, http.StatusOK)
}

func cutoffDate(d time.Duration, override bool) (time.Time, error) {
	if d >= minTTL || override {
		return time.Now().UTC().Add(-d), nil
	}

	return time.Time{}, fmt.Errorf("cleanup ttl %s is less than configured minimum ttl of %s", d, minTTL)
}

type result struct {
	OK     bool     `json:"ok"`
	Errors []string `json:"errors,omitempty"`
}

func respond(w http.ResponseWriter, code int) {
	respondWithError(w, code, nil)
}

func respondWithError(w http.ResponseWriter, code int, err error) {
	var r result
	r.OK = err == nil

	if err != nil {
		var merr *multierror.Error
		if errors.As(err, &merr) {
			for _, err := range merr.WrappedErrors() {
				if err != nil {
					r.Errors = append(r.Errors, err.Error())
				}
			}
		} else {
			r.Errors = append(r.Errors, err.Error())
		}
	}

	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(r)
}
