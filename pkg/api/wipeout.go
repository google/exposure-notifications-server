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
	"net/http"
	"os"
	"time"

	"cambio/pkg/database"
	"cambio/pkg/logging"
)

const (
	ttlEnvVar         = "TTL_DURATION"
	minCutoffDuration = "10d"
)

func HandleWipeout(timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		// Parse and Validate TTL duration string.
		ttlString := os.Getenv(ttlEnvVar)
		ttlDuration, err := getAndValidateDuration(ttlString)
		if err != nil {
			logger.Errorf("TTL env variable error: %v", err)
			http.Error(w, "internal processing error", http.StatusInternalServerError)
			return
		}

		// Parse and Validate min ttl duration string.
		minTtl, err := getAndValidateDuration(minCutoffDuration)
		if err != nil {
			logger.Errorf("min ttl const error: %v", err)
			http.Error(w, "internal processing error", http.StatusInternalServerError)
			return
		}

		// Validate that TTL is sufficiently in the past.
		if ttlDuration < minTtl {
			logger.Errorf("wipeout ttl is less than configured minumum ttl")
			http.Error(w, "internal processing error", http.StatusInternalServerError)
			return
		}

		// Get cutoff timestamp
		cutoff := time.Now().UTC().Add(-ttlDuration)
		logger.Infof("Starting wipeout for records older than %v", cutoff.UTC())

		// Set timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		count, err := database.DeleteInfections(timeoutCtx, cutoff)
		if err != nil {
			logger.Errorf("Failed deleting infections: %v", err)
			http.Error(w, "internal processing error", http.StatusInternalServerError)
			return
		}

		logger.Infof("wipeout run complete, deleted %v records.", count)
		w.WriteHeader(http.StatusOK)
	}
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
