// Copyright 2021 the Exposure Notifications Server authors
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

package jwks

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"go.opencensus.io/stats"
)

const lockID = "jwks-import"

func (s *Server) handleUpdateAll() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), s.config.MaxRuntime)
		defer cancel()

		logger := logging.FromContext(ctx).Named("handleUpdateAll").
			With("lock", lockID)
		logger.Debugw("starting")
		defer logger.Debugw("finishing")

		unlock, err := s.manager.db.Lock(ctx, lockID, time.Minute)
		if err != nil {
			if errors.Is(err, database.ErrAlreadyLocked) {
				logger.Debugw("skipping (already locked)")
				s.h.RenderJSON(w, http.StatusOK, fmt.Errorf("too early"))
				return
			}
			logger.Errorw("unable to obtain lock", "error", err)
			s.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}
		defer func() {
			if err := unlock(); err != nil {
				logger.Errorw("failed to unlock", "error", err)
			}
		}()

		if err := s.manager.UpdateAll(ctx); err != nil {
			logger.Errorw("failed to update all", "error", err)
			s.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}

		stats.Record(ctx, mSuccess.M(1))
		s.h.RenderJSON(w, http.StatusOK, nil)
	})
}
