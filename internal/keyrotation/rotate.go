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

// Package keyrotation implements the API handlers for running key rotation jobs.
package keyrotation

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/revision/database"
	"github.com/hashicorp/go-multierror"
	"go.opencensus.io/trace"
)

// Global lock id for key rotation.
const lockID = "key-rotation-lock"

func (s *Server) handleRotateKeys(ctx context.Context) http.HandlerFunc {
	logger := logging.FromContext(ctx).Named("keyrotation.HandleRotate")

	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := trace.StartSpan(r.Context(), "(*keyrotation.handler).ServeHTTP")
		defer span.End()

		unlock, err := s.db.Lock(ctx, lockID, time.Minute)
		if err != nil {
			logger.Warn(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer func() {
			if err := unlock(); err != nil {
				logger.Errorf("failed to unlock: %v", err)
			}
		}()

		if err := s.doRotate(ctx); err != nil {
			logger.Errorw("failed to rotate", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		logger.Info("key rotation complete.")
		w.WriteHeader(http.StatusOK)
	}
}

func (s *Server) doRotate(ctx context.Context) error {
	metrics := s.env.MetricsExporter(ctx)

	effectiveID, allowed, err := s.revisionDB.GetAllowedRevisionKeys(ctx)
	if err != nil {
		return fmt.Errorf("rotate-keys unable to read revision keys: %w", err)
	}

	// First allowed is newest due to sql orderby.
	var previousCreated time.Time
	if len(allowed) == 0 || time.Since(allowed[0].CreatedAt) >= s.config.NewKeyPeriod {
		key, err := s.revisionDB.CreateRevisionKey(ctx)
		if err != nil {
			return fmt.Errorf("failed to create revision key: %w", err)
		}
		effectiveID = key.KeyID
		previousCreated = key.CreatedAt
		metrics.WriteInt("revision-keys-created", true, 1)
	} else {
		previousCreated = allowed[0].CreatedAt
	}

	var result error
	deleted := 0
	for _, key := range allowed {
		if did, err := s.maybeDeleteKey(ctx, key, effectiveID, previousCreated); err != nil {
			result = multierror.Append(result, err)
		} else if did {
			deleted++
		}
		previousCreated = key.CreatedAt
	}

	metrics.WriteInt("revision-keys-deleted", true, deleted)
	return result
}

func (s *Server) maybeDeleteKey(
	ctx context.Context,
	key *database.RevisionKey,
	effectiveID int64,
	previousCreated time.Time) (bool, error) {

	if key.KeyID == effectiveID {
		return false, nil
	}
	// A key is not safe to delete until the newer one was effective for the period.
	if time.Since(previousCreated) < s.config.DeleteOldKeyPeriod {
		return false, nil
	}
	if err := s.revisionDB.DestroyKey(ctx, key.KeyID); err != nil {
		return false, err
	}
	return true, nil
}
