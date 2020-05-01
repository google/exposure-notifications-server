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
	"errors"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/model"
	"github.com/google/exposure-notifications-server/internal/storage"

	pgx "github.com/jackc/pgx/v4"
)

const (
	bucketEnvVar = "EXPORT_BUCKET"
	oneDay       = 24 * time.Hour
)

// AddExportConfig creates a new ExportConfig record from which batch jobs are created.
func (db *DB) AddExportConfig(ctx context.Context, ec *model.ExportConfig) error {
	if ec.Period > oneDay {
		return errors.New("maximum period is 24h")
	}
	if int64(oneDay.Seconds())%int64(ec.Period.Seconds()) != 0 {
		return errors.New("period must divide equally into 24 hours (e.g., 2h, 4h, 12h, 15m, 30m)")
	}

	var thru *time.Time
	if !ec.Thru.IsZero() {
		thru = &ec.Thru
	}
	err := db.inTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO
				ExportConfig
				(filename_root, period_seconds, include_regions, exclude_regions, from_timestamp, thru_timestamp)
			VALUES
				($1, $2, $3, $4, $5, $6)
			RETURNING config_id
		`, ec.FilenameRoot, int(ec.Period.Seconds()), ec.IncludeRegions, ec.ExcludeRegions, ec.From, thru)

		if err := row.Scan(&ec.ConfigID); err != nil {
			return fmt.Errorf("fetching config_id: %v", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// ExportConfigIterator iterates over a set of export configs.
type ExportConfigIterator interface {
	// Next returns an export config and a flag indicating if the iterator is done (the config will be nil when done==true).
	Next() (infection *model.ExportConfig, done bool, err error)
	// Close should be called when done iterating.
	Close() error
}

// IterateExportConfigs returns an ExportConfigIterator to iterate the ExportConfigs.
func (db *DB) IterateExportConfigs(ctx context.Context, now time.Time) (ExportConfigIterator, error) {
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring connection: %v", err)
	}
	// We don't defer Release() here because the iterator's Close() method will do it.

	rows, err := conn.Query(ctx, `
		SELECT
			config_id, filename_root, period_seconds, include_regions, exclude_regions, from_timestamp, thru_timestamp
		FROM
			ExportConfig
		WHERE
			from_timestamp < $1
			AND
			(thru_timestamp IS NULL OR thru_timestamp > $1)
		`, now)
	if err != nil {
		return nil, err
	}

	return &postgresExportConfigIterator{rows: rows}, nil
}

type postgresExportConfigIterator struct {
	rows pgx.Rows
}

func (i *postgresExportConfigIterator) Next() (*model.ExportConfig, bool, error) {
	if i.rows == nil {
		return nil, true, nil
	}

	if !i.rows.Next() {
		return nil, true, nil
	}

	var m model.ExportConfig
	var periodSeconds int
	var thru *time.Time
	if err := i.rows.Scan(&m.ConfigID, &m.FilenameRoot, &periodSeconds, &m.IncludeRegions, &m.ExcludeRegions, &m.From, &thru); err != nil {
		return nil, false, err
	}
	m.Period = time.Duration(periodSeconds) * time.Second
	if thru != nil {
		m.Thru = *thru
	}
	return &m, false, nil
}

func (i *postgresExportConfigIterator) Close() error {
	if i.rows != nil {
		i.rows.Close()
	}
	return nil
}

// LatestExportBatchEnd returns the end time of the most recent ExportBatch
// for a given ExportConfig. Minimum time (i.e., time.Time{}) is returned
// if no previous ExportBatch exists.
func (db *DB) LatestExportBatchEnd(ctx context.Context, ec *model.ExportConfig) (time.Time, error) {
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return time.Time{}, fmt.Errorf("acquiring connection: %v", err)
	}
	defer conn.Release()

	row := conn.QueryRow(ctx, `
		SELECT
			end_timestamp
		FROM
			ExportBatch
		WHERE
		    config_id = $1
		ORDER BY
		    end_timestamp DESC
		`, ec.ConfigID)

	var latestEnd time.Time
	if err := row.Scan(&latestEnd); err != nil {
		if err == pgx.ErrNoRows {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("scanning result: %v", err)
	}
	return latestEnd, nil
}

// AddExportBatches inserts new export batches.
func (db *DB) AddExportBatches(ctx context.Context, batches []*model.ExportBatch) error {
	err := db.inTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		const stmtName = "insert export batches"
		_, err := tx.Prepare(ctx, stmtName, `
			INSERT INTO
				ExportBatch
				(config_id, filename_root, start_timestamp, end_timestamp, include_regions, exclude_regions, status)
			VALUES
				($1, $2, $3, $4, $5, $6, $7)
		`)
		if err != nil {
			return err
		}

		for _, eb := range batches {
			if _, err := tx.Exec(ctx, stmtName,
				eb.ConfigID, eb.FilenameRoot, eb.StartTimestamp, eb.EndTimestamp, eb.IncludeRegions, eb.ExcludeRegions, eb.Status); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// LeaseBatch returns a leased ExportBatch for the worker to process. If no work to do, nil will be returned.
func (db *DB) LeaseBatch(ctx context.Context, ttl time.Duration, now time.Time) (*model.ExportBatch, error) {
	// Get a set of candidate batchIDs.
	var openBatchIDs []int64
	err := func() error { // Use a function to scope the defer conn.Release().
		conn, err := db.pool.Acquire(ctx)
		if err != nil {
			return fmt.Errorf("acquiring connection: %v", err)
		}
		defer conn.Release()

		// Query for batches that are OPEN or PENDING with expired lease. Also, only return batches with end timestamp
		// in the past (i.e., the batch is complete).
		rows, err := conn.Query(ctx, `
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
			LIMIT 100
		`, model.ExportBatchOpen, model.ExportBatchPending, now)
		if err != nil {
			return err
		}

		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			openBatchIDs = append(openBatchIDs, id)
		}
		return nil
	}()
	if err != nil {
		return nil, err
	}

	if len(openBatchIDs) == 0 {
		return nil, nil
	}

	// Randomize openBatchIDs so that workers aren't competing for the same job.
	openBatchIDs = shuffle(openBatchIDs)

	for _, bid := range openBatchIDs {

		// In a serialized transaction, fetch the existing batch and make sure it can be leased, then lease it.
		leased := false
		err := db.inTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
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

			// If batch is completed or pending (with expiry in the future), this batch no longer available.
			if status == model.ExportBatchComplete || (expires != nil && status == model.ExportBatchPending && now.Before(*expires)) {
				return nil
			}

			_, err = tx.Exec(ctx, `
				UPDATE
					ExportBatch
				SET
					status = $1, lease_expires = $2
				WHERE
				    batch_id = $3
				`, model.ExportBatchPending, now.Add(ttl), bid)
			if err != nil {
				return err
			}
			leased = true
			return nil
		})
		if err != nil {
			return nil, err
		}

		if leased {
			eb, err := db.LookupExportBatch(ctx, bid)
			if err != nil {
				return nil, err
			}
			return eb, nil
		}
	}
	return nil, nil
}

// LookupExportBatch returns an ExportBatch for the given batchID.
func (db *DB) LookupExportBatch(ctx context.Context, batchID int64) (*model.ExportBatch, error) {
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring connection: %v", err)
	}
	defer conn.Release()

	return lookupExportBatch(ctx, batchID, conn.QueryRow)
}

func lookupExportBatch(ctx context.Context, batchID int64, queryRow queryRowFn) (*model.ExportBatch, error) {
	row := queryRow(ctx, `
		SELECT
			batch_id, config_id, filename_root, start_timestamp, end_timestamp, include_regions, exclude_regions, status, lease_expires
		FROM
			ExportBatch
		WHERE
			batch_id = $1
		`, batchID)

	var expires *time.Time
	eb := model.ExportBatch{}
	if err := row.Scan(&eb.BatchID, &eb.ConfigID, &eb.FilenameRoot, &eb.StartTimestamp, &eb.EndTimestamp, &eb.IncludeRegions, &eb.ExcludeRegions, &eb.Status, &expires); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if expires != nil {
		eb.LeaseExpires = *expires
	}
	return &eb, nil
}

// completeBatch marks a batch as completed.
func completeBatch(ctx context.Context, tx pgx.Tx, batchID int64) (err error) {
	batch, err := lookupExportBatch(ctx, batchID, tx.QueryRow)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ErrNotFound
		}
		return err
	}

	if batch.Status == model.ExportBatchComplete {
		return fmt.Errorf("batch %d is already marked completed", batchID)
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

func shuffle(vals []int64) []int64 {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ret := make([]int64, len(vals))
	perm := r.Perm(len(vals))
	for i, randIndex := range perm {
		ret[i] = vals[randIndex]
	}
	return ret
}

func (db *DB) CompleteFileAndBatch(ctx context.Context, files []string, batchID int64, batchCount int) error {
	err := db.inTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		// Update ExportFile for the files created.
		for _, file := range files {
			ef := model.ExportFile{
				Filename: file,
				BatchID:  batchID,
				Region:   "", // TODO(lmohanan) figure out where region comes from.
				BatchNum: batchCount,
				Status:   model.ExportBatchComplete,
			}
			if err := addExportFile(ctx, tx, &ef); err != nil {
				return fmt.Errorf("adding export file entry: %v", err)
			}
		}

		// Update ExportFile for the batch to mark it complete.
		if err := completeBatch(ctx, tx, batchID); err != nil {
			return fmt.Errorf("marking batch %v complete: %v", batchID, err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

type joinedExportBatchFile struct {
	filename    string
	batchID     int64
	count       int
	fileStatus  string
	batchStatus string
}

// DeleteFilesBefore deletes the export batch files for batches ending before the time passed in.
func (db *DB) DeleteFilesBefore(ctx context.Context, before time.Time) (int, error) {
	logger := logging.FromContext(ctx)
	bucket := os.Getenv(bucketEnvVar) // TODO: this should be in the api layer.
	count := 0
	// ReadCommitted is sufficient here because we are working on historical, immutable rows.
	err := db.inTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		// Fetch filenames for  batches where atleast one file is not deleted yet.
		q := `
			SELECT
				ExportBatch.batch_id,
				ExportBatch.status,
				ExportFile.filename,
				ExportFile.batch_size,
				ExportFile.status
			FROM ExportBatch INNER JOIN ExportFile
			WHERE ExportBatch.end_timestamp < $1
			and ExportBatch.status != $2
			and ExportBatch.batch_id = ExportFile.batch_id`
		rows, err := tx.Query(ctx, q, before, model.ExportBatchDeleted)
		if err != nil {
			return fmt.Errorf("fetching filenames: %v", err)
		}
		defer rows.Close()

		batchFileDeleteCounter := make(map[int64]int)
		for rows.Next() {
			var f joinedExportBatchFile
			err := rows.Scan(&f.batchID, &f.batchStatus, &f.filename, &f.count, &f.fileStatus)
			if err != nil {
				return fmt.Errorf("fetching filenames: %v", err)
			}
			defer rows.Close()

			for rows.Next() {
				var f joinedExportBatchFile
				if err := rows.Scan(&f.batchID, &f.batchStatus, &f.filename, &f.count, &f.fileStatus); err != nil {
					return fmt.Errorf("fetching batch_id: %v", err)
				}

				// If file is already deleted, skip to the next
				if f.fileStatus == model.ExportBatchDeleted {
					batchFileDeleteCounter[f.batchID]++
					continue
				}

				// Attempt to delete file
				if err := storage.DeleteObject(ctx, bucket, f.filename); err != nil {
					return fmt.Errorf("delete object: %v", err)
				}

				// Update Status in ExportFile
				if err := updateExportFileStatus(ctx, tx, f.filename, model.ExportBatchDeleted); err != nil {
					return fmt.Errorf("updating ExportFile: %v", err)
				}

				// If batch completely deleted, update in ExportBatch
				if batchFileDeleteCounter[f.batchID] == f.count {
					err = updateExportBatchStatus(ctx, tx, f.batchID, model.ExportBatchDeleted)
					if err != nil {
						return fmt.Errorf("updating ExportBatch: %v", err)
					}

					logger.Infof("Deleted filename %v", f.filename)
					count++
				}

				if err = rows.Err(); err != nil {
					return fmt.Errorf("fetching export files: %v", err)
				}
			}

			logger.Infof("Deleted filename %v", f.filename)
			// commit = true
			count++
		}

		if err = rows.Err(); err != nil {
			return fmt.Errorf("fetching export files: %v", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

func addExportFile(ctx context.Context, tx pgx.Tx, ef *model.ExportFile) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO 
			ExportFile
		    (filename, batch_id, region, batch_num, batch_size, status)
		VALUES
			($1, $2, $3, $4, $5, $6)
		`, ef.Filename, ef.BatchID, ef.Region, ef.BatchNum, ef.BatchSize, ef.Status)
	if err != nil {
		return fmt.Errorf("inserting to ExportFile: %v", err)
	}
	return nil
}

func updateExportFileStatus(ctx context.Context, tx pgx.Tx, filename, status string) error {
	_, err := tx.Exec(ctx, `
		UPDATE ExportFile
		SET
			status = $1
		WHERE
			filename = $2
		`, status, filename)
	if err != nil {
		return fmt.Errorf("updating ExportFile: %v", err)
	}
	return nil
}

func updateExportBatchStatus(ctx context.Context, tx pgx.Tx, batchID int64, status string) error {
	_, err := tx.Exec(ctx, `
		UPDATE ExportBatch
		SET
			status = $1
		WHERE
			batch_id = $2
		`, status, batchID)
	if err != nil {
		return fmt.Errorf("updating ExportBatch: %v", err)
	}
	return nil
}
