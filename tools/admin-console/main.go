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

// This tool provides a small admin UI. Requires connection to the database
// and permissions to access whatever else you might need to access.
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/google/exposure-notifications-server/internal/setup"
)

func main() {
	ctx := context.Background()

	var config Config
	env, closer, err := setup.Setup(ctx, &config)
	if err != nil {
		log.Fatalf("setup.Setup: %v", err)
	}
	defer closer()

	http.Handle("/", NewIndexHandler(&config, env))
	http.Handle("/app", NewAppHandler(&config, env))

	log.Printf("listening on http://localhost:" + config.Port)
	log.Fatal(http.ListenAndServe(":"+config.Port, nil))
}
