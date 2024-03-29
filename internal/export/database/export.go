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

// Package database is a database interface to export.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/exposure-notifications-server/internal/export/model"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/google/exposure-notifications-server/pkg/cryptorand"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/logging"

	pgx "github.com/jackc/pgx/v4"
)

type ExportDB struct {
	db *database.DB
}

func New(db *database.DB) *ExportDB {
	return &ExportDB{
		db: db,
	}
}

// AddExportConfig creates a new ExportConfig record from which batch jobs are created.
func (db *ExportDB) AddExportConfig(ctx context.Context, ec *model.ExportConfig) error {
	if err := ec.Validate(); err != nil {
		return err
	}

	thru := database.NullableTime(ec.Thru)
	return db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO
				ExportConfig
				(bucket_name, filename_root, period_seconds, output_region, from_timestamp, thru_timestamp,
				 signature_info_ids, input_regions, include_travelers, exclude_regions, only_non_travelers,
				 max_records_override)
			VALUES
				($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING config_id
		`, ec.BucketName, ec.FilenameRoot, int(ec.Period.Seconds()), ec.OutputRegion,
			ec.From, thru, ec.SignatureInfoIDs, ec.InputRegions, ec.IncludeTravelers,
			ec.ExcludeRegions, ec.OnlyNonTravelers, ec.MaxRecordsOverride)

		if err := row.Scan(&ec.ConfigID); err != nil {
			return fmt.Errorf("fetching config_id: %w", err)
		}

		return nil
	})
}

// UpdateExportConfig updates an existing ExportConfig record from which batch jobs are created.
func (db *ExportDB) UpdateExportConfig(ctx context.Context, ec *model.ExportConfig) error {
	if err := ec.Validate(); err != nil {
		return err
	}

	thru := database.NullableTime(ec.Thru)
	return db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE
				ExportConfig
			SET
				bucket_name = $1, filename_root = $2, period_seconds = $3, output_region = $4, from_timestamp = $5,
				thru_timestamp = $6, signature_info_ids = $7, input_regions = $8, include_travelers = $9,
				exclude_regions = $10, only_non_travelers = $11, max_records_override = $12
			WHERE config_id = $13
		`, ec.BucketName, ec.FilenameRoot, int(ec.Period.Seconds()), ec.OutputRegion,
			ec.From, thru, ec.SignatureInfoIDs, ec.InputRegions, ec.IncludeTravelers,
			ec.ExcludeRegions, ec.OnlyNonTravelers, ec.MaxRecordsOverride,
			ec.ConfigID)
		if err != nil {
			return fmt.Errorf("updating signatureinfo: %w", err)
		}
		if result.RowsAffected() != 1 {
			return fmt.Errorf("no rows updated")
		}
		return nil
	})
}

func (db *ExportDB) GetExportConfig(ctx context.Context, id int64) (*model.ExportConfig, error) {
	var config *model.ExportConfig
	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT
				config_id, bucket_name, filename_root, period_seconds, output_region,
				from_timestamp, thru_timestamp, signature_info_ids, input_regions,
				include_travelers, exclude_regions, only_non_travelers, max_records_override
			FROM
				ExportConfig
			WHERE
				config_id = $1
	`, id)

		var err error
		config, err = scanOneExportConfig(row)
		if err != nil {
			return fmt.Errorf("failed to parse: %w", err)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("get export config: %w", err)
	}

	return config, nil
}

func (db *ExportDB) GetAllExportConfigs(ctx context.Context) ([]*model.ExportConfig, error) {
	var configs []*model.ExportConfig

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				config_id, bucket_name, filename_root, period_seconds, output_region,
				from_timestamp, thru_timestamp, signature_info_ids, input_regions, include_travelers,
				exclude_regions, only_non_travelers, max_records_override
			FROM
				ExportConfig
			ORDER BY config_id
		`)
		if err != nil {
			return fmt.Errorf("failed to list: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}

			config, err := scanOneExportConfig(rows)
			if err != nil {
				return fmt.Errorf("failed to parse: %w", err)
			}
			configs = append(configs, config)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("get all export configs: %w", err)
	}

	return configs, nil
}

// IterateExportConfigs applies f to each ExportConfig whose FromTimestamp is
// before the given time. If f returns a non-nil error, the iteration stops, and
// the returned error will match f's error with errors.Is.
func (db *ExportDB) IterateExportConfigs(ctx context.Context, t time.Time, f func(*model.ExportConfig) error) (err error) {
	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				config_id, bucket_name, filename_root, period_seconds, output_region,
				from_timestamp, thru_timestamp, signature_info_ids, input_regions, include_travelers,
				exclude_regions, only_non_travelers, max_records_override
			FROM
				ExportConfig
			WHERE
				from_timestamp < $1
			AND
				(thru_timestamp IS NULL OR thru_timestamp > $1)
		`, t)
		if err != nil {
			return fmt.Errorf("failed to list: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}

			m, err := scanOneExportConfig(rows)
			if err != nil {
				return err
			}
			if err = f(m); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return fmt.Errorf("iterate export configs: %w", err)
	}

	return nil
}

func scanOneExportConfig(row pgx.Row) (*model.ExportConfig, error) {
	var (
		m             model.ExportConfig
		outputRegion  sql.NullString
		periodSeconds int
		thru          *time.Time
	)
	if err := row.Scan(&m.ConfigID, &m.BucketName, &m.FilenameRoot, &periodSeconds, &outputRegion, &m.From, &thru,
		&m.SignatureInfoIDs, &m.InputRegions, &m.IncludeTravelers, &m.ExcludeRegions, &m.OnlyNonTravelers, &m.MaxRecordsOverride); err != nil {
		return nil, err
	}

	m.Period = time.Duration(periodSeconds) * time.Second
	if thru != nil {
		m.Thru = *thru
	}

	if outputRegion.Valid {
		m.OutputRegion = outputRegion.String
	}

	return &m, nil
}

func (db *ExportDB) AddSignatureInfo(ctx context.Context, si *model.SignatureInfo) error {
	if si.SigningKey == "" {
		return fmt.Errorf("signing key cannot be empty for a signature info")
	}

	var thru *time.Time
	if !si.EndTimestamp.IsZero() {
		thru = &si.EndTimestamp
	}
	return db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO
 				SignatureInfo
				(signing_key, signing_key_version, signing_key_id, thru_timestamp)
			VALUES
				($1, $2, $3, $4)
			RETURNING id
			`, si.SigningKey, si.SigningKeyVersion, si.SigningKeyID, thru)

		if err := row.Scan(&si.ID); err != nil {
			return fmt.Errorf("fetching id: %w", err)
		}
		return nil
	})
}

func (db *ExportDB) UpdateSignatureInfo(ctx context.Context, si *model.SignatureInfo) error {
	var thru *time.Time
	if !si.EndTimestamp.IsZero() {
		thru = &si.EndTimestamp
	}
	return db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE SignatureInfo
			SET
				signing_key = $1,
				signing_key_version = $2,
				signing_key_id = $3,
				thru_timestamp = $4
			WHERE
				id = $5
 			`, si.SigningKey, si.SigningKeyVersion, si.SigningKeyID, thru, si.ID)
		if err != nil {
			return fmt.Errorf("updating signatureinfo: %w", err)
		}
		if result.RowsAffected() != 1 {
			return fmt.Errorf("no rows updated")
		}
		return nil
	})
}

func (db *ExportDB) ListAllSignatureInfos(ctx context.Context) ([]*model.SignatureInfo, error) {
	var sigs []*model.SignatureInfo

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				id, signing_key, signing_key_version, signing_key_id, thru_timestamp
			FROM
				SignatureInfo
			ORDER BY signing_key_id ASC, signing_key_version ASC, thru_timestamp DESC
		`)
		if err != nil {
			return fmt.Errorf("failed to list: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}

			sig, err := scanOneSignatureInfo(rows)
			if err != nil {
				return fmt.Errorf("failed to parse: %w", err)
			}
			sigs = append(sigs, sig)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("list signature infos: %w", err)
	}

	return sigs, nil
}

func (db *ExportDB) LookupSignatureInfos(ctx context.Context, ids []int64, validUntil time.Time) ([]*model.SignatureInfo, error) {
	var sigs []*model.SignatureInfo

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				id, signing_key, signing_key_version, signing_key_id, thru_timestamp
			FROM
				SignatureInfo
			WHERE
				id = any($1) AND (thru_timestamp is NULL OR thru_timestamp >= $2)
			ORDER BY
				id DESC
		`, ids, validUntil)
		if err != nil {
			return fmt.Errorf("failed to list: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}

			sig, err := scanOneSignatureInfo(rows)
			if err != nil {
				return fmt.Errorf("failed to parse: %w", err)
			}
			sigs = append(sigs, sig)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("lookup signature infos: %w", err)
	}

	return sigs, nil
}

// GetSignatureInfo looks up a single signature info by ID.
func (db *ExportDB) GetSignatureInfo(ctx context.Context, id int64) (*model.SignatureInfo, error) {
	var sig *model.SignatureInfo

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT
				id, signing_key, signing_key_version, signing_key_id, thru_timestamp
			FROM
				SignatureInfo
			WHERE
				id = $1
		`, id)

		var err error
		sig, err = scanOneSignatureInfo(row)
		if err != nil {
			return fmt.Errorf("failed to parse: %w", err)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("get signature info: %w", err)
	}

	return sig, nil
}

func scanOneSignatureInfo(row pgx.Row) (*model.SignatureInfo, error) {
	var info model.SignatureInfo
	var thru *time.Time
	if err := row.Scan(&info.ID, &info.SigningKey, &info.SigningKeyVersion, &info.SigningKeyID, &thru); err != nil {
		return nil, err
	}
	if thru != nil {
		info.EndTimestamp = *thru
	}
	return &info, nil
}

// LatestExportBatchEnd returns the end time of the most recent ExportBatch for
// a given ExportConfig. It returns the zero time if no previous ExportBatch
// exists.
func (db *ExportDB) LatestExportBatchEnd(ctx context.Context, ec *model.ExportConfig) (time.Time, error) {
	var t time.Time

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT
				MAX(end_timestamp)
			FROM
				ExportBatch
			WHERE
				config_id = $1
			LIMIT 1
		`, ec.ConfigID)

		var latestEnd sql.NullTime
		if err := row.Scan(&latestEnd); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return fmt.Errorf("failed to scan result: %w", err)
		}

		if !latestEnd.Valid {
			return nil
		}

		t = latestEnd.Time
		return nil
	}); err != nil {
		return t, fmt.Errorf("latest export batch end: %w", err)
	}

	return t, nil
}

// ListLatestExportBatchEnds returns a map of export config IDs to their latest
// batch end times.
func (db *ExportDB) ListLatestExportBatchEnds(ctx context.Context) (map[int64]*time.Time, error) {
	ts := make(map[int64]*time.Time, 8)

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				config_id, MAX(end_timestamp)
			FROM
				ExportBatch
			GROUP BY config_id
		`)
		if err != nil {
			return fmt.Errorf("failed to list: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}

			var configID int64
			var t time.Time
			if err := rows.Scan(&configID, &t); err != nil {
				return fmt.Errorf("failed to scan result: %w", err)
			}
			ts[configID] = &t
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("list latest export batch ends: %w", err)
	}

	return ts, nil
}

// AddExportBatches inserts new export batches.
func (db *ExportDB) AddExportBatches(ctx context.Context, batches []*model.ExportBatch) error {
	return db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		const stmtName = "insert export batches"
		_, err := tx.Prepare(ctx, stmtName, `
			INSERT INTO
				ExportBatch
				(config_id, bucket_name, filename_root, start_timestamp, end_timestamp, output_region, status, signature_info_ids, input_regions, include_travelers, exclude_regions, only_non_travelers, max_records_override)
			VALUES
				($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`)
		if err != nil {
			return err
		}

		for _, eb := range batches {
			if _, err := tx.Exec(ctx, stmtName,
				eb.ConfigID, eb.BucketName, eb.FilenameRoot, eb.StartTimestamp, eb.EndTimestamp, eb.OutputRegion, eb.Status, eb.SignatureInfoIDs,
				eb.InputRegions, eb.IncludeTravelers, eb.ExcludeRegions, eb.OnlyNonTravelers, eb.MaxRecordsOverride); err != nil {
				return err
			}
		}
		return nil
	})
}

// LeaseBatch returns a leased ExportBatch for the worker to process. If no work to do, nil will be returned.
func (db *ExportDB) LeaseBatch(ctx context.Context, ttl time.Duration, batchMaxCloseTime time.Time) (*model.ExportBatch, error) {
	var openBatchIDs []int64

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		// batches are ordered by end time. We try to fill the "oldest"
		// batches first so that padding written will be covered by
		// "newer" export batches.
		rows, err := tx.Query(ctx, `
			SELECT
				batch_id
			FROM
				ExportBatch
			WHERE
			    (
					status = $1
					OR
					(status = $2 AND lease_expires < $3)
				)
			AND
				end_timestamp < $3
			ORDER BY
				end_timestamp ASC
			LIMIT 100
		`, model.ExportBatchOpen, model.ExportBatchPending, batchMaxCloseTime)
		if err != nil {
			return fmt.Errorf("failed to list: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}

			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			openBatchIDs = append(openBatchIDs, id)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("list authorized apps: %w", err)
	}

	if len(openBatchIDs) == 0 {
		return nil, nil
	}

	// Randomize openBatchIDs so that workers aren't competing for the same job.
	shuffle(openBatchIDs)

	for _, bid := range openBatchIDs {
		bid := bid

		// In a serialized transaction, fetch the existing batch and make sure it can be leased, then lease it.
		leased := false
		err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
			row := tx.QueryRow(ctx, `
				SELECT
					status, lease_expires
				FROM
					ExportBatch
				WHERE
					batch_id = $1
				`, bid)

			var status string
			var expires *time.Time
			if err := row.Scan(&status, &expires); err != nil {
				return err
			}

			if status == model.ExportBatchComplete || (expires != nil && status == model.ExportBatchPending && batchMaxCloseTime.Before(*expires)) {
				// Something beat us to this batch, it's no longer available.
				return nil
			}

			if _, err := tx.Exec(ctx, `
				UPDATE
					ExportBatch
				SET
					status = $1, lease_expires = $2
				WHERE
					batch_id = $3
				`,
				model.ExportBatchPending, batchMaxCloseTime.Add(ttl), bid,
			); err != nil {
				return err
			}

			leased = true
			return nil
		})
		if err != nil {
			return nil, err
		}

		if leased {
			return db.LookupExportBatch(ctx, bid)
		}
	}
	// We didn't manage to lease any of the candidates, so return no work to be done (nil).
	return nil, nil
}

// LookupExportBatch returns an ExportBatch for the given batchID.
func (db *ExportDB) LookupExportBatch(ctx context.Context, batchID int64) (*model.ExportBatch, error) {
	var batch *model.ExportBatch

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		var err error
		batch, err = lookupExportBatch(ctx, batchID, tx.QueryRow)
		return err
	}); err != nil {
		return nil, fmt.Errorf("lookup export batch: %w", err)
	}

	return batch, nil
}

type queryRowFn func(ctx context.Context, query string, args ...interface{}) pgx.Row

func lookupExportBatch(ctx context.Context, batchID int64, queryRow queryRowFn) (*model.ExportBatch, error) {
	row := queryRow(ctx, `
		SELECT
			batch_id, config_id, bucket_name, filename_root, start_timestamp, end_timestamp, output_region, status, lease_expires, signature_info_ids, input_regions, include_travelers, exclude_regions, only_non_travelers, max_records_override
		FROM
			ExportBatch
		WHERE
			batch_id = $1
		LIMIT 1
		`, batchID)

	var expires *time.Time
	eb := model.ExportBatch{}
	if err := row.Scan(&eb.BatchID, &eb.ConfigID, &eb.BucketName, &eb.FilenameRoot, &eb.StartTimestamp, &eb.EndTimestamp, &eb.OutputRegion, &eb.Status, &expires, &eb.SignatureInfoIDs, &eb.InputRegions, &eb.IncludeTravelers, &eb.ExcludeRegions, &eb.OnlyNonTravelers, &eb.MaxRecordsOverride); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, database.ErrNotFound
		}
		return nil, err
	}
	if expires != nil {
		eb.LeaseExpires = *expires
	}
	return &eb, nil
}

// FinalizeBatch writes the ExportFile records and marks the ExportBatch as complete.
func (db *ExportDB) FinalizeBatch(ctx context.Context, eb *model.ExportBatch, files []string, batchSize int) error {
	return db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		// Update ExportFile for the files created.
		for i, file := range files {
			ef := model.ExportFile{
				BucketName:       eb.BucketName,
				Filename:         file,
				BatchID:          eb.BatchID,
				OutputRegion:     eb.OutputRegion,
				InputRegions:     eb.InputRegions,
				IncludeTravelers: eb.IncludeTravelers,
				OnlyNonTravelers: eb.OnlyNonTravelers,
				ExcludeRegions:   eb.ExcludeRegions,
				BatchNum:         i + 1,
				BatchSize:        batchSize,
				Status:           model.ExportBatchComplete,
			}
			if err := addExportFile(ctx, tx, &ef); err != nil {
				if errors.Is(err, database.ErrKeyConflict) {
					logging.FromContext(ctx).Infof("ExportFile %q already exists in database, skipping without overwriting. This can occur when reprocessing a failed batch.", file)
				} else {
					return fmt.Errorf("adding export file entry: %w", err)
				}
			}
		}

		// Update ExportBatch to mark it complete.
		if err := completeBatch(ctx, tx, eb.BatchID); err != nil {
			return fmt.Errorf("marking batch %v complete: %w", eb.BatchID, err)
		}
		return nil
	})
}

// MarkExpiredFiles marks files for deletion.
func (db *ExportDB) MarkExpiredFiles(ctx context.Context, configID int64, ttl time.Duration) (int, error) {
	var filesToDelete int
	return filesToDelete, db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		minTime := time.Now().Add(-1 * ttl)
		res, err := tx.Exec(ctx, `
		UPDATE
			ExportFile AS ef
		SET
			status = $5
		FROM
			ExportBatch AS eb
		WHERE
			eb.config_id = $1
		AND
			ef.batch_id = eb.batch_id
		AND
			eb.end_timestamp < $2
		AND
			eb.status = $3
		AND
			ef.status = $4
		`,
			configID, minTime, model.ExportBatchComplete, model.ExportBatchComplete, model.ExportBatchDeletePending)
		if err != nil {
			return fmt.Errorf("updating ExportFile: %w", err)
		}
		filesToDelete = int(res.RowsAffected())
		return nil
	})
}

// LookupExportFiles returns a list of completed and unexpired export files for a specific config.
func (db *ExportDB) LookupExportFiles(ctx context.Context, configID int64, ttl time.Duration) ([]string, error) {
	var files []string

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		minTime := time.Now().Add(-1 * ttl)

		rows, err := tx.Query(ctx, `
			SELECT
				ef.filename
			FROM
				ExportFile ef
			INNER JOIN
				ExportBatch eb ON (eb.batch_id = ef.batch_id)
			WHERE
				eb.config_id = $1
			AND
				eb.start_timestamp > $2
			AND
				(eb.status = $3 OR eb.status = $4)
			AND
				ef.status = $5
			ORDER BY
				ef.filename
		`,
			configID, minTime, model.ExportBatchComplete, model.ExportBatchDeleted, model.ExportBatchComplete,
		)
		if err != nil {
			return fmt.Errorf("failed to list: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}

			var file string
			if err := rows.Scan(&file); err != nil {
				return fmt.Errorf("failed to parse: %w", err)
			}
			files = append(files, file)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("lookup export files: %w", err)
	}

	return files, nil
}

type joinedExportBatchFile struct {
	bucketName  string
	filename    string
	batchID     int64
	count       int
	fileStatus  string
	batchStatus string
}

func (db *ExportDB) LookupExportFile(ctx context.Context, filename string) (*model.ExportFile, error) {
	var file model.ExportFile

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT
				bucket_name, filename, batch_id, output_region, batch_num, batch_size, status, input_regions, include_travelers, exclude_regions, only_non_travelers
			FROM
				ExportFile
			WHERE
				filename = $1
			LIMIT 1
		`, filename)

		if err := row.Scan(&file.BucketName, &file.Filename, &file.BatchID, &file.OutputRegion, &file.BatchNum, &file.BatchSize, &file.Status, &file.InputRegions, &file.IncludeTravelers, &file.ExcludeRegions, &file.OnlyNonTravelers); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return database.ErrNotFound
			}
			return fmt.Errorf("failed to parse: %w", err)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("get authorized app: %w", err)
	}

	return &file, nil
}

// DeleteFilesBefore deletes the export batch files for batches ending before the time passed in.
func (db *ExportDB) DeleteFilesBefore(ctx context.Context, before time.Time, blobstore storage.Blobstore) (int, error) {
	var files []joinedExportBatchFile

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				eb.batch_id,
				eb.status,
				eb.bucket_name,
				ef.filename,
				ef.batch_size,
				ef.status
			FROM
				ExportBatch eb
			INNER JOIN
				ExportFile ef ON (eb.batch_id = ef.batch_id)
			WHERE
				eb.end_timestamp < $1
				AND eb.status != $2
				AND ef.status = $3
		`, before, model.ExportBatchDeleted, model.ExportBatchDeletePending)
		if err != nil {
			return fmt.Errorf("failed to list: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}

			var f joinedExportBatchFile
			if err := rows.Scan(&f.batchID, &f.batchStatus, &f.bucketName, &f.filename, &f.count, &f.fileStatus); err != nil {
				return fmt.Errorf("failed to fetch batch: %w", err)
			}
			files = append(files, f)
		}

		return nil
	}); err != nil {
		return 0, fmt.Errorf("delete files before: %w", err)
	}

	count := 0
	batchFileDeleteCounter := make(map[int64]int)

	for _, f := range files {
		f := f

		// If file is already deleted, skip to the next.
		if f.fileStatus == model.ExportBatchDeleted {
			batchFileDeleteCounter[f.batchID]++
			continue
		}

		// Delete stored file.
		gcsCtx, cancel := context.WithTimeout(ctx, time.Second*50)
		defer cancel()
		if err := blobstore.DeleteObject(gcsCtx, f.bucketName, f.filename); err != nil {
			return 0, fmt.Errorf("delete object: %w", err)
		}

		err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
			// Update Status in ExportFile.
			if err := updateExportFileStatus(ctx, tx, f.filename, model.ExportBatchDeleted); err != nil {
				return fmt.Errorf("updating ExportFile: %w", err)
			}

			// If batch completely deleted, update in ExportBatch.
			if batchFileDeleteCounter[f.batchID] == f.count {
				if err := updateExportBatchStatus(ctx, tx, f.batchID, model.ExportBatchDeleted); err != nil {
					return fmt.Errorf("updating ExportBatch: %w", err)
				}
			}
			return nil
		})
		if err != nil {
			return 0, err
		}

		count++
	}

	return count, nil
}

// addExportFile adds a row to ExportFile. If the row already exists (based on the primary key),
// ErrKeyConflict is returned.
func addExportFile(ctx context.Context, tx pgx.Tx, ef *model.ExportFile) error {
	tag, err := tx.Exec(ctx, `
		INSERT INTO
			ExportFile
			(bucket_name, filename, batch_id, output_region, batch_num, batch_size, status, input_regions, include_travelers, exclude_regions, only_non_travelers)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (filename) DO NOTHING
		`, ef.BucketName, ef.Filename, ef.BatchID, ef.OutputRegion, ef.BatchNum, ef.BatchSize, ef.Status, ef.InputRegions, ef.IncludeTravelers, ef.ExcludeRegions, ef.OnlyNonTravelers)
	if err != nil {
		return fmt.Errorf("inserting to ExportFile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return database.ErrKeyConflict
	}
	return nil
}

func updateExportFileStatus(ctx context.Context, tx pgx.Tx, filename, status string) error {
	_, err := tx.Exec(ctx, `
		UPDATE
			ExportFile
		SET
			status = $1
		WHERE
			filename = $2
		`, status, filename)
	if err != nil {
		return fmt.Errorf("updating ExportFile: %w", err)
	}
	return nil
}

func updateExportBatchStatus(ctx context.Context, tx pgx.Tx, batchID int64, status string) error {
	_, err := tx.Exec(ctx, `
		UPDATE
			ExportBatch
		SET
			status = $1
		WHERE
			batch_id = $2
		`, status, batchID)
	if err != nil {
		return fmt.Errorf("updating ExportBatch: %w", err)
	}
	return nil
}

// completeBatch marks a batch as completed.
func completeBatch(ctx context.Context, tx pgx.Tx, batchID int64) error {
	logger := logging.FromContext(ctx)
	batch, err := lookupExportBatch(ctx, batchID, tx.QueryRow)
	if err != nil {
		return err
	}

	if batch.Status == model.ExportBatchComplete {
		// Batch is already completed.
		logger.Warnf("When completing a batch, the status of batch %d was already %s.", batchID, model.ExportBatchComplete)
		return nil
	}

	_, err = tx.Exec(ctx, `
		UPDATE
			ExportBatch
		SET
			status = $1, lease_expires = NULL
		WHERE
			batch_id = $2
		`, model.ExportBatchComplete, batchID)
	if err != nil {
		return err
	}
	return nil
}

// shuffle shuffles the values in vals in-place.
func shuffle(vals []int64) {
	//nolint:gosec // cryptorand.NewSource is a random source
	r := rand.New(cryptorand.NewSource())
	r.Shuffle(len(vals), func(i, j int) {
		vals[i], vals[j] = vals[j], vals[i]
	})
}
