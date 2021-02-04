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

package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

const (
	// googleCloudTraceHeader is the header with trace data.
	googleCloudTraceHeader = "X-Cloud-Trace-Context"

	// googleCloudTraceKey is the key in the structured log where trace information
	// is expected to be present.
	googleCloudTraceKey = "logging.googleapis.com/trace"
)

// googleCloudProjectID is the project id, populated by Terraform during service
// deployment.
var googleCloudProjectID = os.Getenv("PROJECT_ID")

// PopulateLogger populates the logger onto the context.
func PopulateLogger(originalLogger *zap.SugaredLogger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			logger := originalLogger

			// Only override the logger if it's the default logger. This is only used
			// for testing and is intentionally a strict object equality check because
			// the default logger is a global default in the logger package.
			if existing := logging.FromContext(ctx); existing == logging.DefaultLogger() {
				logger = existing
			}

			// If there's a request ID, set that on the logger.
			if id := RequestIDFromContext(ctx); id != "" {
				logger = logger.With("request_id", id)
			}

			// On Google Cloud, extract the trace context and add it to the logger.
			if v := r.Header.Get(googleCloudTraceHeader); v != "" && googleCloudProjectID != "" {
				parts := strings.Split(v, "/")
				if len(parts) > 0 && len(parts[0]) > 0 {
					val := fmt.Sprintf("projects/%s/traces/%s", googleCloudProjectID, parts[0])
					logger = logger.With(googleCloudTraceKey, val)
				}
			}

			ctx = logging.WithLogger(ctx, logger)
			r = r.Clone(ctx)

			next.ServeHTTP(w, r)
		})
	}
}
