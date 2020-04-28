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
	"fmt"
	"os"
	"time"

	pgx "github.com/jackc/pgx/v4"
)

const (
	statusOpen    = "OPEN"
	statusDone    = "DONE"
	statusDeleted = "DELETED"
	bucketEnvVar  = "EXPORT_BUCKET"
)

func AddNewBatch(ctx context.Context) (err error) {
	logger := logging.FromContext(ctx)
	conn, err := Connection(ctx)
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

	eb, err := calculateNextBatch(ctx, tx)
	if err != nil {
		return fmt.Errorf("calculating batch: %v", err)
	}

	bid, err := insertBatch(ctx, tx, eb)
	if err != nil {
		return fmt.Errorf("inserting batch: %v", err)
	}

	logger.Infof("Inserted batch id %v", bid)

	// TODO(guray): insert into ExportFile with that batch, query for exposure key counts, ...
	commit = true
	return nil
}

func calculateNextBatch(ctx context.Context, tx pgx.Tx) (model.ExportBatch, error) {
	// TODO(guray): lookup end_timestamp of last successful batch, truncate, etc
	return model.ExportBatch{
		StartTimestamp: time.Now().UTC().Add(time.Hour * -24),
		EndTimestamp:   time.Now().UTC(),
		Status:         statusOpen,
	}, nil
}

func insertBatch(ctx context.Context, tx pgx.Tx, b model.ExportBatch) (int64, error) {
	// Postgres sql driver doesn't support LastInsertedId, so using "RETURNING"
	// and scanning the query result
	row := tx.QueryRow(ctx, `
		INSERT INTO ExportBatch
			(start_timestamp, end_timestamp, status)
		VALUES
			($1, $2, $3)
		RETURNING batch_id
		`, b.StartTimestamp, b.EndTimestamp, b.Status)

	var id int64
	if err := row.Scan(&id); err != nil {
		return -1, fmt.Errorf("fetching batch_id: %v", err)
	}
	return id, nil
}

type joinedExportBatchFile struct {
	filename    string
	batchId     int
	count       int
	fileStatus  string
	batchStatus string
}

// DeleteFilesBefore deletes the export batch files for batches ending before the time passed in.
func DeleteFilesBefore(ctx context.Context, before time.Time) (count int, err error) {
	logger := logging.FromContext(ctx)
	conn, err := Connection(ctx)
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
	rows, err := conn.Query(ctx, q, before, statusDeleted)
	if err != nil {
		return 0, fmt.Errorf("fetching filenames: %v", err)
	}
	defer rows.Close()

	bucket := os.Getenv(bucketEnvVar)
	batchFileDeleteCounter := make(map[int]int)
	for rows.Next() {
		var f joinedExportBatchFile
		err := rows.Scan(&f.batchId, &f.batchStatus, &f.filename, &f.count, &f.fileStatus)
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
		if f.fileStatus == statusDeleted {
			batchFileDeleteCounter[f.batchId]++
			commit = true
			continue
		}

		// Attempt to delete file
		err = storage.DeleteObject(bucket, f.filename)
		if err != nil {
			return count, fmt.Errorf("delete object: %v", err)
		}

		// Update Status in ExportFile
		err = updateExportFileStatus(ctx, tx, f.filename, statusDeleted)
		if err != nil {
			return count, fmt.Errorf("updating ExportFile: %v", err)
		}

		// If batch completely deleted, update in ExportBatch
		if batchFileDeleteCounter[f.batchId] == f.count {
			err = updateExportBatchStatus(ctx, tx, f.filename, statusDeleted)
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

func updateExportBatchStatus(ctx context.Context, tx pgx.Tx, batchId string, status string) error {
	_, err := tx.Exec(ctx, `
		UPDATE ExportBatch
		SET
			status = $1
		WHERE
			batch_id = $2
		`, status, batchId)
	if err != nil {
		return fmt.Errorf("updating ExportBatch: %v", err)
	}
	return nil
}
