// Copyright 2021 Google LLC
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
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"go.opencensus.io/stats"
)

const jwksLockID = "jwks-import"

func (s *Server) handleUpdateAll() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx).Named("handleUpdateAll")

		unlock, err := s.manager.db.Lock(ctx, jwksLockID, time.Minute)
		if err != nil {
			if errors.Is(err, database.ErrAlreadyLocked) {
				w.WriteHeader(http.StatusOK)
				return
			}
			logger.Errorw("unable to obtain lock", "lock", jwksLockID, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer func() {
			if err := unlock(); err != nil {
				logger.Errorw("failed to unlock", "lock", jwksLockID, "error", err)
			}
		}()

		if err := s.manager.UpdateAll(ctx); err != nil {
			logger.Errorw("failed to update all", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		stats.Record(ctx, mJWKSSuccess.M(1))
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, http.StatusText(http.StatusOK))
	})
}
