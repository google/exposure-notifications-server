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
	"go.opencensus.io/trace"
)

func (s *Server) handleRotateKeys(ctx context.Context) http.HandlerFunc {
	logger := logging.FromContext(ctx).Named("keyrotation.HandleRotate")

	return func(w http.ResponseWriter, r *http.Request) {
		_, span := trace.StartSpan(r.Context(), "(*keyrotation.handler).ServeHTTP")
		defer span.End()

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
		return fmt.Errorf("rotate-keys unable to read revision keys: %v", err)
	}

	// First allowed is newest due to sql orderby.
	if len(allowed) == 0 || time.Since(allowed[0].CreatedAt) >= s.config.NewKeyPeriod {
		if _, err := s.revisionDB.CreateRevisionKey(ctx); err != nil {
			return err
		}
		metrics.WriteInt("revision-keys-created", true, 1)
	}

	if len(allowed) < 2 {
		return nil
	}

	deleted := 0
	defer metrics.WriteInt("revision-keys-deleted", true, deleted)

	for _, key := range allowed[2:] {
		if time.Since(key.CreatedAt) < s.config.DeleteOldKeyPeriod {
			continue
		}

		if err := s.revisionDB.DestroyKey(ctx, key.KeyID); err != nil {
			return err
		}
		deleted++
	}

	return nil
}
