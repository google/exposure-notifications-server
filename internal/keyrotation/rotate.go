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
	"errors"
	"fmt"
	"net/http"
	"time"

	revisiondatabase "github.com/google/exposure-notifications-server/internal/revision/database"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/hashicorp/go-multierror"
	"go.opencensus.io/stats"
	"go.opencensus.io/trace"
)

// Global lock id for key rotation.
const lockID = "key-rotation-lock"

func (s *Server) handleRotateKeys() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("handleRotate").
			With("lock_id", lockID)

		ctx, span := trace.StartSpan(ctx, "(*keyrotation.handler).ServeHTTP")
		defer span.End()

		unlock, err := s.db.Lock(ctx, lockID, time.Minute)
		if err != nil {
			logger.Warnw("unable to obtain lock", "lock", lockID, "error", err)
			if errors.Is(err, database.ErrAlreadyLocked) {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer func() {
			if err := unlock(); err != nil {
				logger.Errorw("failed to unlock", "lock", lockID, "error", err)
			}
		}()

		if err := s.doRotate(ctx); err != nil {
			logger.Errorw("failed to rotate", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		stats.Record(ctx, mRotationSuccess.M(1))
		logger.Info("key rotation complete")
		w.WriteHeader(http.StatusOK)
	})
}

func (s *Server) doRotate(ctx context.Context) error {
	logger := logging.FromContext(ctx).Named("keyrotation.doRotate")

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
		logger.Info("Created new revision key.")
		stats.Record(ctx, mRevisionKeysCreated.M(1))
	} else {
		previousCreated = allowed[0].CreatedAt
	}

	var result *multierror.Error
	deleted := 0
	for _, key := range allowed {
		if did, err := s.maybeDeleteKey(ctx, key, effectiveID, previousCreated); err != nil {
			result = multierror.Append(result, err)
		} else if did {
			deleted++
		}
		previousCreated = key.CreatedAt
	}
	if deleted > 0 {
		logger.Infof("Deleted %d old revision keys.", deleted)
		stats.Record(ctx, mRevisionKeysDeleted.M(int64(deleted)))
	}
	return result.ErrorOrNil()
}

func (s *Server) maybeDeleteKey(ctx context.Context, key *revisiondatabase.RevisionKey, effectiveID int64, previousCreated time.Time) (bool, error) {
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
