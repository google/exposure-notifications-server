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

// Package api defines the structures for the infection publishing API.
package api

import (
	"net/http"
	"time"

	"cambio/pkg/database"
	"cambio/pkg/logging"
	"cambio/pkg/model"
	"cambio/pkg/verification"
)

func HandlePublish() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		var data model.Publish
		err, code := unmarshal(w, r, &data)
		if err != nil {
			logger.Errorf("error unmarhsaling API call, code: %v: %v", code, err)
			// Log but don't return internal decode error message reason.
			http.Error(w, "bad API request", code)
			return
		}

		err = verification.VerifySafetyNet(ctx)
		if err != nil {
			logger.Errorf("unable to verify safetynet payload: %v", err)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		batchTime := time.Now().UTC()
		infections, err := model.TransformPublish(&data, batchTime)
		if err != nil {
			logger.Errorf("error transforming publish data: %v", err)
			http.Error(w, "bad API request", http.StatusBadRequest)
			return
		}

		err = database.InsertInfections(ctx, infections)
		if err != nil {
			logger.Errorf("error writing infection record: %v", err)
			http.Error(w, "internal processing error", http.StatusInternalServerError)
			return
		}
		logger.Infof("Inserted %d infections.", len(infections))

		w.WriteHeader(http.StatusOK)
	}
}
