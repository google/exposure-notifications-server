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
	"github.com/hashicorp/go-multierror"
	"go.opencensus.io/trace"
)

func (s *Server) handleRotateKeys(ctx context.Context) http.HandlerFunc {
	logger := logging.FromContext(ctx).Named("keyrotation.HandleRotate")

	return func(w http.ResponseWriter, r *http.Request) {
		_, span := trace.StartSpan(r.Context(), "(*keyrotation.handler).ServeHTTP")
		defer span.End()

		// TODO(whaught): This mutex should be a DB lock. Doesn't help for many instances.
		s.mu.Lock()
		defer s.mu.Unlock()

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

	_, allowed, err := s.revisionDB.GetAllowedRevisionKeys(ctx)
	if err != nil {
		return fmt.Errorf("rotate-keys unable to read revision keys: %w", err)
	}

	// First allowed is newest due to sql orderby.
	if len(allowed) == 0 || time.Since(allowed[0].CreatedAt) >= s.config.NewKeyPeriod {
		if _, err := s.revisionDB.CreateRevisionKey(ctx); err != nil {
			return fmt.Errorf("failed to create revision key: %w", err)
		}
		metrics.WriteInt("revision-keys-created", true, 1)
	}

	var result error
	deleted := 0
	for _, key := range allowed {
		if time.Since(key.CreatedAt) < s.config.DeleteOldKeyPeriod {
			continue
		}
		if err := s.revisionDB.DestroyKey(ctx, key.KeyID); err != nil {
			result = multierror.Append(result, err)
			continue
		}
		deleted++
	}

	metrics.WriteInt("revision-keys-deleted", true, deleted)
	return result
}
