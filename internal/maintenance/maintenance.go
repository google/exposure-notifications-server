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

// Package maintenance provides utilities for maintenance mode handling
package maintenance

import (
	"fmt"
	"net/http"
)

// Config is an interface that determines if
// the implementer can supply maintenance mode settings.
type Config interface {
	MaintenanceMode() bool
}

// Responder is a handle to a configured maintenance mode responder.
type Responder struct {
	inMaintenance bool
}

// New creates a new maintenance mode responder.
func New(c Config) *Responder {
	return &Responder{
		inMaintenance: c.MaintenanceMode(),
	}
}

// Handle will either return the maintenance mode responder (if enabled)
// or pass through to the next handler.
func (r *Responder) Handle(next http.Handler) http.Handler {
	if r.inMaintenance {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprintf(w, "{\"error\": \"please try again later\"}")
		})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}
