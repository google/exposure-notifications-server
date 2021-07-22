// Copyright 2020 the Exposure Notifications Server authors
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

// Package database is a database interface to publish.
package database

import (
	"context"
	"fmt"

	"github.com/google/exposure-notifications-server/internal/publish/model"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"

	pgx "github.com/jackc/pgx/v4"
)

func executeBulkInsertExposure(ctx context.Context, tx pgx.Tx, expos []*model.Exposure) (int64, error) {
	inputRows := [][]interface{}{}
	for _, exp := range expos {
		if exp.ReportType == verifyapi.ReportTypeNegative {
			continue
		}
		var syncID *int64
		if exp.FederationSyncID != 0 {
			syncID = &exp.FederationSyncID
		}
		inputRows = append(inputRows, []interface{}{
			encodeExposureKey(exp.ExposureKey), exp.TransmissionRisk,
			exp.AppPackageName, exp.Regions, exp.Traveler, exp.IntervalNumber, exp.IntervalCount,
			exp.CreatedAt, exp.LocalProvenance, syncID,
			exp.HealthAuthorityID, exp.ReportType, exp.DaysSinceSymptomOnset,
		})
	}

	copyCount, err := tx.CopyFrom(ctx, pgx.Identifier{"public", "exposure"},
		[]string{
			"exposure_key",
			"transmission_risk",
			"app_package_name",
			"regions",
			"traveler",
			"interval_number",
			"interval_count",
			"created_at",
			"local_provenance",
			"sync_id",
			"health_authority_id",
			"report_type",
			"days_since_symptom_onset",
		}, pgx.CopyFromRows(inputRows))
	if err != nil {
		return 0, fmt.Errorf("unexpected error for Bulk Insert: %w", err)
	}
	if int(copyCount) != len(inputRows) {
		return 0, fmt.Errorf("expected Bulk Insert to return %d copied rows, but got %d", len(inputRows), copyCount)
	}
	return copyCount, nil
}

// BulkInsertExposures performs a large key copy at once allowing for quick population of exposure keys
// *this should NOT be used for anything but testing, validation and checks were removed
func (db *PublishDB) BulkInsertExposures(ctx context.Context, incoming []*model.Exposure) (int, error) {
	var updated int
	err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		count, err := executeBulkInsertExposure(ctx, tx, incoming)
		if err != nil {
			return err
		}
		updated = int(count)
		return nil
	})
	if err != nil {
		updated = 0
	}
	return updated, err
}
