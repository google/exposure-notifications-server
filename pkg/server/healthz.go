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

package server

import (
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/pkg/logging"
)

func HandleHealthz(db *database.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("server.HandleHealthz")

		conn, err := db.Pool.Acquire(ctx)
		if err != nil {
			logger.Errorw("failed to acquire database connection", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError)
			return
		}
		defer conn.Release()

		if err := conn.Conn().Ping(ctx); err != nil {
			logger.Errorw("failed to ping database", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError)
			return
		}

		w.WriteHeader(200)
		fmt.Fprintf(w, `{"status": "ok"}`)
	})
}
