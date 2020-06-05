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

	coredb "github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/export/database"
	"github.com/google/exposure-notifications-server/internal/export/model"
	"github.com/google/exposure-notifications-server/internal/setup"
)

var (
	bucketName        = flag.String("bucket-name", "", "The bucket name to store the export file.")
	filenameRoot      = flag.String("filename-root", "", "The root filename for the export file.")
	period            = flag.Duration("period", 24*time.Hour, "The frequency with which to create export files.")
	region            = flag.String("region", "", "The output region for the export batches/files.")
	fromTimestamp     = flag.String("from-timestamp", "", "The timestamp (RFC3339) when this config becomes active.")
	thruTimestamp     = flag.String("thru-timestamp", "", "The timestamp (RFC3339) when this config ends.")
	signingKey        = flag.String("signing-key", "", "The KMS resource ID to use for signing batches.")
	signingKeyID      = flag.String("signing-key-id", "", "The ID of the signing key (for clients).")
	signingKeyVersion = flag.String("signing-key-version", "", "The version of the signing key (for clients).")
)

func main() {
	flag.Parse()

	if *bucketName == "" {
		log.Fatal("--bucket-name is required.")
	}
	if *filenameRoot == "" {
		log.Fatal("--filename-root is required.")
	}
	if *region == "" {
		log.Fatal("--region is required.")
	}
	*region = strings.ToUpper(*region)

	fromTime := time.Now()
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

	if *signingKey == "" {
		log.Printf("WARNING - you are creating an export config without a signing key!!")
	}

	ctx := context.Background()
	var config coredb.Config
	env, err := setup.Setup(ctx, &config)
	if err != nil {
		log.Fatalf("failed to setup: %v", err)
	}
	defer env.Close(ctx)

	db := database.New(env.Database())

	si := model.SignatureInfo{
		SigningKey:        *signingKey,
		SigningKeyVersion: *signingKeyVersion,
		SigningKeyID:      *signingKeyID,
	}
	if err := db.AddSignatureInfo(ctx, &si); err != nil {
		log.Fatalf("AddSignatureInfo: %v", err)
	}

	ec := model.ExportConfig{
		BucketName:       *bucketName,
		FilenameRoot:     *filenameRoot,
		Period:           *period,
		OutputRegion:     *region,
		From:             fromTime,
		Thru:             thruTime,
		SignatureInfoIDs: []int64{si.ID},
	}
	if err := db.AddExportConfig(ctx, &ec); err != nil {
		log.Fatalf("Failure: %v", err)
	}
	log.Printf("Successfully created ExportConfig %d.", ec.ConfigID)
}
