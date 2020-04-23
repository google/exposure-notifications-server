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

	"cambio/pkg/database"
	"cambio/pkg/logging"
	"cambio/pkg/pb"
	"cambio/pkg/storage"
	"fmt"
	"strconv"
	"time"

	"github.com/golang/protobuf/proto"
)

func HandleExport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		limit := 30000
		limits, ok := r.URL.Query()["limit"]
		if ok && len(limits) > 0 {
			lim, err := strconv.Atoi(limits[0])
			if err == nil {
				limit = lim
			}
		}
		logger.Infof("limiting to %v", limit)
		// TODO(guray): split into multiple batches
		criteria := database.IterateInfectionsCriteria{
			// TODO(guray): calculate Since/UntilTimestamp's based on last success
			SinceTimestamp: time.Now().UTC().AddDate(0, 0, -5),
			UntilTimestamp: time.Now().UTC(),
			// TODO(guray): use IncludeRegions to split into multiple files
			OnlyLocalProvenance: false, // include federated ids
		}
		it, err := database.IterateInfections(ctx, criteria)
		if err != nil {
			logger.Errorf("error getting infections: %v", err)
			http.Error(w, "internal processing error", http.StatusInternalServerError)
			return
		}
		defer it.Close()
		inf, done, err := it.Next()
		if err != nil {
			logger.Errorf("error getting infections: %v", err)
			http.Error(w, "internal processing error", http.StatusInternalServerError)
			return
		}
		var exposureKeys []*pb.ExposureKeyExport_ExposureKey
		num := 1
		for !done && err == nil && num <= limit {
			exposureKey := pb.ExposureKeyExport_ExposureKey{
				ExposureKey:    inf.ExposureKey,
				IntervalNumber: inf.IntervalNumber,
				IntervalCount:  inf.IntervalCount,
			}
			exposureKeys = append(exposureKeys, &exposureKey)
			num++
			inf, done, err = it.Next()
		}
		if err != nil {
			logger.Errorf("error getting infections: %v", err)
			http.Error(w, "internal processing error", http.StatusInternalServerError)
			return
		}
		batch := pb.ExposureKeyExport{
			// TODO(guray): real metadata, depending on what batch this is
			StartTimestamp: time.Now().UTC().Unix(),
			Region:         "US",
			Keys:           exposureKeys,
		}
		data, err := proto.Marshal(&batch)
		if err != nil {
			logger.Errorf("error serializing proto: %v", err)
			http.Error(w, "internal processing error", http.StatusInternalServerError)
		}
		// TODO(guray): sort out naming scheme, cache control, etc
		objectName := fmt.Sprintf("testExport-%d-records.pb", limit)
		if err := storage.CreateObject("apollo-public-bucket", objectName, data); err != nil {
			logger.Errorf("error creating cloud storage object: %v", err)
			http.Error(w, "internal processing error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
