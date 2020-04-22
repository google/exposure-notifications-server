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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"cambio/pkg/database"
	"cambio/pkg/logging"
	"cambio/pkg/model"

	"cloud.google.com/go/datastore"
)

// Scans the database and writes descrypted diagnosis keys to the terminal.
// This is to verify the crypto operations.
func main() {
	var numKeys = flag.Int("num", 1, "number of keys to scan -num=1")
	flag.Parse()

	ctx := context.Background()
	logger := logging.FromContext(ctx)

	if err := database.Initialize(); err != nil {
		logger.Fatalf("database.Initialize: %v", err)
	}

	client := database.Connection()
	if client == nil {
		logger.Fatalf("database.Connection error")
	}

	var infections []model.Infection
	q := datastore.NewQuery("infection").Order("- createdAt").Limit(*numKeys)
	if _, err := client.GetAll(ctx, q, &infections); err != nil {
		logger.Fatalf("unable to query datbase: %v", err)
	}

	var stdout = os.Stdout
	for _, inf := range infections {
		fmt.Fprintf(stdout, "%v | %v\n", inf.K, inf.ExposureKey)
	}
}
