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
		cutoff := time.Now().Add(-ttlDuration)
		logger.Infof("Starting wipeout for records older than %v", cutoff.UTC())

		// Set timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		// Get wipeout keys older than cutoff timestamp
		wipeoutKeys, err := database.FilterKeysOnly(timeoutCtx, cutoff)
		if err != nil {
			logger.Errorf("error getting wipeout keys: %v", err)
			// TODO(lmohanan): Work out error codes depending on cloud run retry behavior
			http.Error(w, "internal processing error", http.StatusInternalServerError)
			return
		}
		numKeys := len(wipeoutKeys)
		logger.Infof("%v records match", numKeys)

		if numKeys == 0 {
			logger.Infof("nothing to wipeout")
			w.WriteHeader(http.StatusOK)
			return
		}

		// Delete wipeout keys older than cutoff
		count, err := database.DeleteDiagnosisKeys(timeoutCtx, wipeoutKeys)
		if err != nil {
			logger.Errorf("error completing wipeout: %v", err)
			if count == 0 {
				// TODO(lmohanan): Work out error codes depending on cloud run retry behavior
				http.Error(w, "internal processing error", http.StatusInternalServerError)
				return
			}

			if count < numKeys {
				// TODO(lmohanan): Work out error codes depending on cloud run retry behavior
				http.Error(w, "partial success", http.StatusMultiStatus)
				return
			}
		}

		if timeoutCtx.Err() != nil && timeoutCtx.Err() == context.DeadlineExceeded {
			logger.Infof("wipeout run timed out at %v.", timeout)
			return
		}

		//TODO(lmohanan) add a metric for key count and deleted count.
		logger.Infof("wipeout run complete, deleted %v records of %v records", count, numKeys)
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
