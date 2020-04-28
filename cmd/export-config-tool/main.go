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
	"cambio/pkg/database"
	cflag "cambio/pkg/flag"
	"cambio/pkg/model"
	"context"
	"flag"
	"log"
	"time"
)

var (
	filenameRoot  = flag.String("filename-root", "", "The root filename for the export file.")
	period        = flag.Duration("period", 24*time.Hour, "The frequency with which to create export files.")
	fromTimestamp = flag.String("from-timestamp", "", "The timestamp (RFC3339) when this config becomes active.")
	thruTimestamp = flag.String("thru-timestamp", "", "The timestamp (RFC3339) when this config ends.")
)

func main() {
	var includeRegions, excludeRegions cflag.RegionListVar
	flag.Var(&includeRegions, "regions", "A comma-separated list of regions to query. Leave blank for all regions.")
	flag.Var(&excludeRegions, "exclude-regions", "A comma-separated list fo regions to exclude from the query.")
	flag.Parse()

	if *filenameRoot == "" {
		log.Fatal("--filename-root is required.")
	}

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
	db, err := database.NewFromEnv(ctx)
	if err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}
	defer db.Close(ctx)

	ec := model.ExportConfig{
		FilenameRoot:   *filenameRoot,
		Period:         *period,
		IncludeRegions: includeRegions,
		ExcludeRegions: excludeRegions,
		From:           fromTime,
		Thru:           thruTime,
	}

	if err := db.AddExportConfig(ctx, &ec); err != nil {
		log.Fatalf("Failure: %v", err)
	}
	log.Printf("Successfully created ExportConfig %d.", ec.ConfigID)
}
