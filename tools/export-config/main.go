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

// This package is used to create entries in the ExportConfig table. Each ExportConfig entry is used to create rows in the ExportBatch table.
package main

import (
	"context"
	"flag"
	"log"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/internal/model"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/serverenv"
)

var (
	filenameRoot  = flag.String("filename-root", "", "The root filename for the export file.")
	period        = flag.Duration("period", 24*time.Hour, "The frequency with which to create export files.")
	region        = flag.String("region", "", "The region for the export batches/files.")
	fromTimestamp = flag.String("from-timestamp", "", "The timestamp (RFC3339) when this config becomes active.")
	thruTimestamp = flag.String("thru-timestamp", "", "The timestamp (RFC3339) when this config ends.")
)

func main() {
	flag.Parse()

	if *filenameRoot == "" {
		log.Fatal("--filename-root is required.")
	}
	if *region == "" {
		log.Fatal("--region is required.")
	}
	*region = strings.ToUpper(*region)

	fromTime := time.Now().UTC()
	if *fromTimestamp != "" {
		var err error
		fromTime, err = time.Parse(time.RFC3339, *fromTimestamp)
		if err != nil {
			log.Fatalf("Failed to parse --from-timestamp (use RFC3339): %v", err)
		}
	}

	var thruTime time.Time
	if *thruTimestamp != "" {
		var err error
		thruTime, err = time.Parse(time.RFC3339, *thruTimestamp)
		if err != nil {
			log.Fatalf("Failed to parse --thru-timestamp (use RFC3339): %v", err)
		}
	}

	ctx := context.Background()
	// It is possible to install a different secret management system here that conforms to secrets.SecretManager{}
	sm, err := secrets.NewGCPSecretManager(ctx)
	if err != nil {
		log.Fatalf("unable to connect to secret manager: %v", err)
	}
	env := serverenv.New(ctx,
		serverenv.WithSecretManager(sm),
		serverenv.WithMetricsExporter(metrics.NewLogsBasedFromContext))

	db, err := database.NewFromEnv(ctx, env)
	if err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}
	defer db.Close(ctx)

	ec := model.ExportConfig{
		FilenameRoot: *filenameRoot,
		Period:       *period,
		Region:       *region,
		From:         fromTime,
		Thru:         thruTime,
	}

	if err := db.AddExportConfig(ctx, &ec); err != nil {
		log.Fatalf("Failure: %v", err)
	}
	log.Printf("Successfully created ExportConfig %d.", ec.ConfigID)
}
