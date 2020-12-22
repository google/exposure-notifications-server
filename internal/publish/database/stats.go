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

package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/exposure-notifications-server/internal/publish/model"
	pgx "github.com/jackc/pgx/v4"
)

// ReadStats will return all stats before the current hour, ordered in ascending time.
func (db *PublishDB) ReadStats(ctx context.Context, healthAuthorityID int64) ([]*model.HealthAuthorityStats, error) {
	if healthAuthorityID <= 0 {
		return nil, fmt.Errorf("missing healthAuthorityID")
	}

	// Slightly larger than normal upper bound.
	results := make([]*model.HealthAuthorityStats, 0, 15*24)
	// This time is only used to init structures, and immediately overridden from the database read.
	now := time.Now().UTC()

	err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
		SELECT
			health_authority_id, hour, publish, teks, revisions, oldest_tek_days, onset_age_days, missing_onset
		FROM
			HealthAuthorityStats
		WHERE
			health_authority_id=$1
		ORDER BY hour ASC
		`, healthAuthorityID)
		if err != nil {
			return fmt.Errorf("read stats: %w", err)
		}

		for rows.Next() {
			// The time the hour is initialized to doesn't matter, the scan will override it.
			stats := model.InitHour(healthAuthorityID, now)
			if err := scanOneHealthAuthorityStats(rows, stats); err != nil {
				return err
			}
			results = append(results, stats)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return results, nil
}

func scanOneHealthAuthorityStats(rows pgx.Rows, stats *model.HealthAuthorityStats) error {
	return rows.Scan(
		&stats.HealthAuthorityID, &stats.Hour, &stats.PublishCount, &stats.TEKCount,
		&stats.RevisionCount, &stats.OldestTekDays, &stats.OnsetAgeDays, &stats.MissingOnset)
}

// UpdateStats performance a read-modify-write to update the requested stats.
func (db *PublishDB) UpdateStats(ctx context.Context, hour time.Time, healthAuthorityID int64, info *model.PublishInfo) error {
	return db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		return db.UpdateStatsInTx(ctx, tx, hour, healthAuthorityID, info)
	})
}

// UpdateStatsInTx records information for the health authority during a publish request, in an existing database transaction.
func (db *PublishDB) UpdateStatsInTx(ctx context.Context, tx pgx.Tx, hour time.Time, healthAuthorityID int64, info *model.PublishInfo) error {
	if healthAuthorityID <= 0 {
		return nil
	}

	hour = hour.UTC().Truncate(time.Hour)

	rows, err := tx.Query(ctx, `
		SELECT
			health_authority_id, hour, publish, teks, revisions, oldest_tek_days, onset_age_days, missing_onset
		FROM
			HealthAuthorityStats
		WHERE
			health_authority_id=$1 AND hour=$2
		FOR UPDATE
		`, healthAuthorityID, hour)
	if err != nil {
		return fmt.Errorf("read stats: %w", err)
	}

	stats := model.InitHour(healthAuthorityID, hour)
	if rows.Next() {
		// This hour already exists, so load it.
		if err := scanOneHealthAuthorityStats(rows, stats); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan stats: %w", err)
		}
	}
	// Thw rows.close here isn't deferred because it needs to be closed before the exec below.
	rows.Close()

	stats.AddPublish(info)

	// Insert/Update the stats for this hour.
	_, err = tx.Exec(ctx, `
		INSERT INTO
			HealthAuthorityStats
			(health_authority_id, hour, publish, teks, revisions, oldest_tek_days, onset_age_days, missing_onset)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (health_authority_id, hour) DO
			UPDATE
			SET publish=$3, teks=$4, revisions=$5, oldest_tek_days=$6, onset_age_days=$7, missing_onset=$8
		`,
		stats.HealthAuthorityID, stats.Hour, stats.PublishCount, stats.TEKCount, stats.RevisionCount,
		stats.OldestTekDays, stats.OnsetAgeDays, stats.MissingOnset)
	if err != nil {
		return fmt.Errorf("update stats: %w", err)
	}

	return nil
}
