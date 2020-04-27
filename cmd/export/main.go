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

// This package is the service that publishes infected keys; it is intended to be invoked over HTTP by Cloud Scheduler.
package main

import (
	"context"
	"log"
	"net/http"

	"cambio/pkg/api"
	"cambio/pkg/database"
	"cambio/pkg/logging"

	"github.com/gorilla/mux"
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	cleanup, err := database.Initialize(ctx)
	if err != nil {
		logger.Fatalf("unable to connect to database: %v", err)
	}
	defer cleanup(ctx)

	router := mux.NewRouter()
	// TODO(guray): remove or gate the /test handler
	router.HandleFunc("/test", api.HandleTestExport())
	router.HandleFunc("/setupBatch", api.HandleSetupBatch())
	router.HandleFunc("/pollWork", api.HandlePollWork())
	logger.Info("starting infection export server")
	log.Fatal(http.ListenAndServe(":8080", router))
}
