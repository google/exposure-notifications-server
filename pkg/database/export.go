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
	"cambio/pkg/logging"
	"cambio/pkg/model"
	"cambio/pkg/storage"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"time"

	pgx "github.com/jackc/pgx/v4"
)

const (
	bucketEnvVar = "EXPORT_BUCKET"
	oneDay       = 24 * time.Hour
)

// AddExportConfig creates a new ExportConfig record from which batch jobs are created.
func (db *DB) AddExportConfig(ctx context.Context, ec *model.ExportConfig) (err error) {
	if ec.Period > oneDay {
		return errors.New("maximum period is 24h")
	}
	if int64(oneDay.Seconds())%int64(ec.Period.Seconds()) != 0 {
		return errors.New("period must divide equally into 24 hours (e.g., 2h, 4h, 12h, 15m, 30m)")
	}

	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("unable to obtain database connection: %v", err)
	}
	defer conn.Release()

	commit := false
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("starting transaction: %v", err)
	}
	defer finishTx(ctx, tx, &commit, &err)

	var thru *time.Time
	if !ec.Thru.IsZero() {
		thru = &ec.Thru
	}
	row := tx.QueryRow(ctx, `
		INSERT INTO ExportConfig
			(filename_root, period_seconds, include_regions, exclude_regions, from_timestamp, thru_timestamp)
		VALUES
			($1, $2, $3, $4, $5, $6)
		RETURNING config_id
		`, ec.FilenameRoot, int(ec.Period.Seconds()), ec.IncludeRegions, ec.ExcludeRegions, ec.From, thru)

	if err := row.Scan(&ec.ConfigID); err != nil {
		return fmt.Errorf("fetching config_id: %v", err)
	}

	commit = true
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
		return nil, fmt.Errorf("unable to obtain database connection: %v", err)
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

// LatestExportBatchStartTime returns the most recent ExportBatch start time
// for a given ExportConfig. nil will be returned if batch has never been
// created.
func (db *DB) LatestExportBatchStartTime(ctx context.Context, ec *model.ExportConfig) (*time.Time, error) {
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to obtain database connection: %v", err)
	}
	defer conn.Release()

	row := conn.QueryRow(ctx, `
		SELECT
			start_timestamp
		FROM
			ExportBatch
		WHERE
		    config_id = $1
		ORDER BY
		    start_timestamp DESC
		`, ec.ConfigID)

	var latestStart time.Time
	if err := row.Scan(&latestStart); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scanning result: %v", err)
	}
	if latestStart.IsZero() {
		return nil, nil
	}
	return &latestStart, nil
}

// AddExportBatch adds a new export batch for the given ExportConfig.
func (db *DB) AddExportBatch(ctx context.Context, eb *model.ExportBatch) (err error) {
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("unable to obtain database connection: %v", err)
	}
	defer conn.Release()

	commit := false
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("starting transaction: %v", err)
	}
	defer finishTx(ctx, tx, &commit, &err)

	row := tx.QueryRow(ctx, `
		INSERT INTO ExportBatch
			(config_id, filename_root, start_timestamp, end_timestamp, include_regions, exclude_regions, status)
		VALUES
			($1, $2, $3, $4, $5, $6, $7)
		RETURNING batch_id
		`, eb.ConfigID, eb.FilenameRoot, eb.StartTimestamp, eb.EndTimestamp, eb.IncludeRegions, eb.ExcludeRegions, eb.Status)

	if err := row.Scan(&eb.BatchID); err != nil {
		return fmt.Errorf("fetching batch_id: %v", err)
	}

	commit = true
	return nil
}

// LeaseBatch returns a leased ExportBatch for the worker to process. If no work to do, nil will be returned.
func (db *DB) LeaseBatch(ctx context.Context, ttl time.Duration, now time.Time) (eb *model.ExportBatch, err error) {
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to obtain database connection: %v", err)
	}
	defer conn.Release()

	// Query for batches that are OPEN or PENDING with expired lease. Also, only return batches with end timestamp
	// in the past (i.e., the batch is complete).
	// TODO(jasonco): Should we wait even a bit longer to allow the batch to be completely full?
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
		return nil, err
	}

	var openBatchIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		openBatchIDs = append(openBatchIDs, id)
	}

	if len(openBatchIDs) == 0 {
		return nil, nil
	}

	// Randomize openBatchIDs so that workers aren't competing for the same job.
	openBatchIDs = shuffle(openBatchIDs)

	for _, bid := range openBatchIDs {

		// In a serialized transaction, fetch the existing batch and make sure it can be reserved, then reserve it.
		done, err := func() (done bool, err error) {

			commit := false
			tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
			if err != nil {
				return false, err
			}
			defer finishTx(ctx, tx, &commit, &err)

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
				return false, err
			}
			if status == model.ExportBatchComplete || (expires != nil && status == model.ExportBatchPending && now.Before(*expires)) {
				return false, nil
			}

			_, err = tx.Exec(ctx, `
					UPDATE ExportBatch
					SET
						status = $1, lease_expires = $2
					WHERE
					    batch_id = $3
					`, model.ExportBatchPending, now.Add(ttl), bid)
			if err != nil {
				return false, err
			}
			commit = true
			return true, nil

		}()
		if err != nil {
			return nil, err
		}
		if done {
			eb, err := lookupExportBatch(ctx, bid, conn.QueryRow)
			if err != nil {
				return nil, err
			}
			return eb, nil
		}
	}
	return nil, nil
}

// // LookupExportBatch
// func LookupExportBatch(ctx context.Context, batchID int64) (*model.ExportBatch, error) {
// 	conn, err := Connection(ctx)
// 	if err != nil {
// 		return nil, fmt.Errorf("unable to obtain database connection: %v", err)
// 	}
// 	defer conn.Release()
// 	return lookupExportBatch(ctx, batchID, conn.Query)
// }

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
		return nil, err
	}
	if expires != nil {
		eb.LeaseExpires = *expires
	}
	return &eb, nil
}

// CompleteBatch marks a batch as completed.
func (db *DB) CompleteBatch(ctx context.Context, batchID int64) (err error) {
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("unable to obtain database connection: %v", err)
	}
	defer conn.Release()

	commit := false
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer finishTx(ctx, tx, &commit, &err)

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

	commit = true
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

type joinedExportBatchFile struct {
	filename    string
	batchID     int
	count       int
	fileStatus  string
	batchStatus string
}

// DeleteFilesBefore deletes the export batch files for batches ending before the time passed in.
func (db *DB) DeleteFilesBefore(ctx context.Context, before time.Time) (count int, err error) {
	logger := logging.FromContext(ctx)
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return 0, fmt.Errorf("unable to obtain database connection: %v", err)
	}
	defer conn.Release()

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
	rows, err := conn.Query(ctx, q, before, model.ExportBatchDeleted)
	if err != nil {
		return 0, fmt.Errorf("fetching filenames: %v", err)
	}
	defer rows.Close()

	bucket := os.Getenv(bucketEnvVar)
	batchFileDeleteCounter := make(map[int]int)
	for rows.Next() {
		var f joinedExportBatchFile
		err := rows.Scan(&f.batchID, &f.batchStatus, &f.filename, &f.count, &f.fileStatus)
		if err != nil {
			return count, fmt.Errorf("fetching batch_id: %v", err)
		}

		// Begin transaction
		commit := false
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
		if err != nil {
			return count, fmt.Errorf("starting transaction: %v", err)
		}
		defer finishTx(ctx, tx, &commit, &err)

		// If file is already deleted, skip to the next
		if f.fileStatus == model.ExportBatchDeleted {
			batchFileDeleteCounter[f.batchID]++
			commit = true
			continue
		}

		// Attempt to delete file
		err = storage.DeleteObject(ctx, bucket, f.filename)
		if err != nil {
			return count, fmt.Errorf("delete object: %v", err)
		}

		// Update Status in ExportFile
		err = updateExportFileStatus(ctx, tx, f.filename, model.ExportBatchDeleted)
		if err != nil {
			return count, fmt.Errorf("updating ExportFile: %v", err)
		}

		// If batch completely deleted, update in ExportBatch
		if batchFileDeleteCounter[f.batchID] == f.count {
			err = updateExportBatchStatus(ctx, tx, f.filename, model.ExportBatchDeleted)
			if err != nil {
				return count, fmt.Errorf("updating ExportBatch: %v", err)
			}
		}

		logger.Infof("Deleted filename %v", f.filename)
		commit = true
		count++
	}

	if err = rows.Err(); err != nil {
		return count, fmt.Errorf("fetching export files: %v", err)
	}
	return count, nil
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

func updateExportBatchStatus(ctx context.Context, tx pgx.Tx, batchID string, status string) error {
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
