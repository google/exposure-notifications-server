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

// Package cleanup implements the API handlers for running data deletion jobs.
package cleanup

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/go-multierror"
)

const minTTL = 10 * 24 * time.Hour

func cutoffDate(d time.Duration, override bool) (time.Time, error) {
	if d >= minTTL || override {
		return time.Now().UTC().Add(-d), nil
	}

	return time.Time{}, fmt.Errorf("cleanup ttl %s is less than configured minimum ttl of %s", d, minTTL)
}

type result struct {
	OK     bool     `json:"ok"`
	Errors []string `json:"errors,omitempty"`
}

func respond(w http.ResponseWriter, code int) {
	respondWithError(w, code, nil)
}

func respondWithError(w http.ResponseWriter, code int, err error) {
	var r result
	r.OK = err == nil

	if err != nil {
		var merr *multierror.Error
		if errors.As(err, &merr) {
			for _, err := range merr.WrappedErrors() {
				if err != nil {
					r.Errors = append(r.Errors, err.Error())
				}
			}
		} else {
			r.Errors = append(r.Errors, err.Error())
		}
	}

	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(r)
}
