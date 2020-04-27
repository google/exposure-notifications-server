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
	"cambio/pkg/model"
	"cambio/pkg/storage"
	"context"
	"fmt"
	"strconv"
	"time"
)

func SetupBatchHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)
	logger.Infof("setting up tables for next batch...")
	database.AddNewBatch(ctx)
	w.WriteHeader(http.StatusOK)
}

func PollWorkHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)
	logger.Infof("Polling for open work...")
	w.WriteHeader(http.StatusOK)
}

func TestExportHandler(w http.ResponseWriter, r *http.Request) {
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
	since := time.Now().UTC().AddDate(0, 0, -5)
	until := time.Now().UTC()
	exposureKeys, err := queryExposureKeys(ctx, since, until, limit)
	if err != nil {
		logger.Errorf("error getting infections: %v", err)
		http.Error(w, "internal processing error", http.StatusInternalServerError)
	}
	data, err := MarshalExportFile(since, until, exposureKeys, "US")
	if err != nil {
		logger.Errorf("error marshalling export file: %v", err)
		http.Error(w, "internal processing error", http.StatusInternalServerError)
	}
	objectName := fmt.Sprintf("testExport-%d-records.pb", limit)
	if err := storage.CreateObject("apollo-public-bucket", objectName, data); err != nil {
		logger.Errorf("error creating cloud storage object: %v", err)
		http.Error(w, "internal processing error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func queryExposureKeys(ctx context.Context, since, until time.Time, limit int) ([]*model.Infection, error) {
	criteria := database.IterateInfectionsCriteria{
		SinceTimestamp:      since,
		UntilTimestamp:      until,
		OnlyLocalProvenance: false, // include federated ids
	}
	it, err := database.IterateInfections(ctx, criteria)
	if err != nil {
		return nil, err
	}
	defer it.Close()
	var exposureKeys []*model.Infection
	num := 1
	exp, done, err := it.Next()
	for !done && err == nil && num <= limit {
		if exp != nil {
			exposureKeys = append(exposureKeys, exp)
			num++
		}
		exp, done, err = it.Next()
	}
	if err != nil {
		return nil, err
	}
	return exposureKeys, nil
}
