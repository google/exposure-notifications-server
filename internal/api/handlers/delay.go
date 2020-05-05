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

// Package handlers provide common utilities for wrapping HTTP handlers.
package handlers

import (
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/internal/logging"
)

// WithMinimumLatency wrapps the passed in http handler func and ensures a minimum target duration is reached.
func WithMinimumLatency(h http.Handler, target time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetTime := time.Now().Add(target)
		h.ServeHTTP(w, r)

		currentTime := time.Now()
		if !currentTime.After(targetTime) {
			wait := targetTime.Sub(currentTime)
			select {
			case <-time.After(wait):
			case <-r.Context().Done():
				logging.FromContext(r.Context()).Errorf("context cancelled before response could be sent")
				return
			}
		}
	}
}
