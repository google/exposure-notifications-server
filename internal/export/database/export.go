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
	"math/rand"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/export/model"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/storage"

	pgx "github.com/jackc/pgx/v4"
)

const (
	bucketEnvVar = "EXPORT_BUCKET"
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

	var thru *time.Time
	if !ec.Thru.IsZero() {
		thru = &ec.Thru
	}
	return db.db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO
				ExportConfig
				(bucket_name, filename_root, period_seconds, region, from_timestamp, thru_timestamp, signature_info_ids)
			VALUES
				($1, $2, $3, $4, $5, $6, $7)
			RETURNING config_id
		`, ec.BucketName, ec.FilenameRoot, int(ec.Period.Seconds()), ec.Region,
			ec.From, thru, ec.SignatureInfoIDs)

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

	var thru *time.Time
	if !ec.Thru.IsZero() {
		thru = &ec.Thru
	}
	return db.db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE
				ExportConfig
			SET
				bucket_name = $1, filename_root = $2, period_seconds = $3, region = $4, from_timestamp = $5, thru_timestamp = $6, signature_info_ids = $7
			WHERE config_id = $8
		`, ec.BucketName, ec.FilenameRoot, int(ec.Period.Seconds()), ec.Region,
			ec.From, thru, ec.SignatureInfoIDs, ec.ConfigID)
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
	conn, err := db.db.Pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring connection: %w", err)
	}
	defer conn.Release()

	row := conn.QueryRow(ctx, `
		SELECT
			config_id, bucket_name, filename_root, period_seconds, region, from_timestamp, thru_timestamp, signature_info_ids
		FROM
			ExportConfig
		WHERE
			config_id = $1`, id)

	return scanOneExportConfig(row)
}

func (db *ExportDB) GetAllExportConfigs(ctx context.Context) ([]*model.ExportConfig, error) {
	conn, err := db.db.Pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring connection: %w", err)
	}
	defer conn.Release()

	rows, err := conn.Query(ctx, `
		SELECT
			config_id, bucket_name, filename_root, period_seconds, region, from_timestamp, thru_timestamp, signature_info_ids
		FROM
			ExportConfig`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []*model.ExportConfig{}
	for rows.Next() {
		ec, err := scanOneExportConfig(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, ec)
	}

	return results, nil
}

// IterateExportConfigs applies f to each ExportConfig whose FromTimestamp is
// before the given time. If f returns a non-nil error, the iteration stops, and
// the returned error will match f's error with errors.Is.
func (db *ExportDB) IterateExportConfigs(ctx context.Context, t time.Time, f func(*model.ExportConfig) error) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("IterateExportConfigs(%s): %w", t, err)
		}
	}()

	conn, err := db.db.Pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquiring connection: %w", err)
	}
	defer conn.Release()

	rows, err := conn.Query(ctx, `
		SELECT
			config_id, bucket_name, filename_root, period_seconds, region, from_timestamp, thru_timestamp, signature_info_ids
		FROM
			ExportConfig
		WHERE
			from_timestamp < $1
			AND
			(thru_timestamp IS NULL OR thru_timestamp > $1)
		`, t)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		m, err := scanOneExportConfig(rows)
		if err != nil {
			return err
		}
		if err = f(m); err != nil {
			return err
		}
	}
	return rows.Err()
}

func scanOneExportConfig(row pgx.Row) (*model.ExportConfig, error) {
	var (
		m             model.ExportConfig
		periodSeconds int
		thru          *time.Time
	)
	if err := row.Scan(&m.ConfigID, &m.BucketName, &m.FilenameRoot, &periodSeconds, &m.Region, &m.From, &thru, &m.SignatureInfoIDs); err != nil {
		return nil, err
	}
	m.Period = time.Duration(periodSeconds) * time.Second
	if thru != nil {
		m.Thru = *thru
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
	return db.db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO
 				SignatureInfo
				(signing_key, app_package_name, bundle_id, signing_key_version, signing_key_id, thru_timestamp)
			VALUES
				($1, $2, $3, $4, $5, $6)
			RETURNING id
			`, si.SigningKey, si.AppPackageName, si.BundleID, si.SigningKeyVersion, si.SigningKeyID, thru)

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
	return db.db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE SignatureInfo
			SET
				signing_key = $1, app_package_name = $2, bundle_id = $3,
				signing_key_version = $4, signing_key_id = $5, thru_timestamp = $6
			WHERE
				id = $7
 			`, si.SigningKey, si.AppPackageName, si.BundleID, si.SigningKeyVersion, si.SigningKeyID, thru, si.ID)
		if err != nil {
			return fmt.Errorf("updating signatureinfo: %w", err)
		}
		if result.RowsAffected() != 1 {
			return fmt.Errorf("no rows updated")
		}
		return nil
	})
}

func (db *ExportDB) ListAllSigntureInfos(ctx context.Context) ([]*model.SignatureInfo, error) {
	conn, err := db.db.Pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring connection: %w", err)
	}
	defer conn.Release()

	rows, err := conn.Query(ctx, `
    SELECT
      id, signing_key, app_package_name, bundle_id, signing_key_version, signing_key_id, thru_timestamp
    FROM
      SignatureInfo
    ORDER BY signing_key_id ASC, signing_key_version ASC, thru_timestamp DESC, app_package_name ASC, bundle_id ASC
  `)
	if err != nil {
		return nil, err
	}

	var sigInfos []*model.SignatureInfo
	for rows.Next() {
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		si, err := scanOneSignatureInfo(ctx, rows)
		if err != nil {
			return nil, err
		}
		sigInfos = append(sigInfos, si)
	}

	return sigInfos, nil
}

func (db *ExportDB) LookupSignatureInfos(ctx context.Context, ids []int64, validUntil time.Time) ([]*model.SignatureInfo, error) {
	conn, err := db.db.Pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring connection: %w", err)
	}
	defer conn.Release()

	rows, err := conn.Query(ctx, `
    SELECT
      id, signing_key, app_package_name, bundle_id, signing_key_version, signing_key_id, thru_timestamp
    FROM
      SignatureInfo
    WHERE
      id = any($1) AND (thru_timestamp is NULL OR thru_timestamp >= $2)
  `, ids, validUntil)
	if err != nil {
		return nil, err
	}

	var sigInfos []*model.SignatureInfo
	for rows.Next() {
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		si, err := scanOneSignatureInfo(ctx, rows)
		if err != nil {
			return nil, err
		}
		sigInfos = append(sigInfos, si)
	}

	return sigInfos, nil
}

// GetSignatureInfo looks up a single signature info by ID.
func (db *ExportDB) GetSignatureInfo(ctx context.Context, id int64) (*model.SignatureInfo, error) {
	conn, err := db.db.Pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring connection: %w", err)
	}
	defer conn.Release()

	row := conn.QueryRow(ctx, `
		SELECT
			id, signing_key, app_package_name, bundle_id, signing_key_version, signing_key_id, thru_timestamp
		FROM
			SignatureInfo
		WHERE
			id = $1
		`, id)

	return scanOneSignatureInfo(ctx, row)
}

func scanOneSignatureInfo(ctx context.Context, row pgx.Row) (*model.SignatureInfo, error) {
	var info model.SignatureInfo
	var thru *time.Time
	if err := row.Scan(&info.ID, &info.SigningKey, &info.AppPackageName, &info.BundleID, &info.SigningKeyVersion, &info.SigningKeyID, &thru); err != nil {
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
// TODO(squee1945): This needs a
func (db *ExportDB) LatestExportBatchEnd(ctx context.Context, ec *model.ExportConfig) (time.Time, error) {
	conn, err := db.db.Pool.Acquire(ctx)
	if err != nil {
		return time.Time{}, fmt.Errorf("acquiring connection: %w", err)
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
		LIMIT 1
		`, ec.ConfigID)

	var latestEnd time.Time
	if err := row.Scan(&latestEnd); err != nil {
		if err == pgx.ErrNoRows {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("scanning result: %w", err)
	}
	return latestEnd, nil
}

// AddExportBatches inserts new export batches.
func (db *ExportDB) AddExportBatches(ctx context.Context, batches []*model.ExportBatch) error {
	return db.db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		const stmtName = "insert export batches"
		_, err := tx.Prepare(ctx, stmtName, `
			INSERT INTO
				ExportBatch
				(config_id, bucket_name, filename_root, start_timestamp, end_timestamp, region, status, signature_info_ids)
			VALUES
				($1, $2, $3, $4, $5, $6, $7, $8)
		`)
		if err != nil {
			return err
		}

		for _, eb := range batches {
			if _, err := tx.Exec(ctx, stmtName,
				eb.ConfigID, eb.BucketName, eb.FilenameRoot, eb.StartTimestamp, eb.EndTimestamp, eb.Region, eb.Status, eb.SignatureInfoIDs); err != nil {
				return err
			}
		}
		return nil
	})
}

// LeaseBatch returns a leased ExportBatch for the worker to process. If no work to do, nil will be returned.
func (db *ExportDB) LeaseBatch(ctx context.Context, ttl time.Duration, now time.Time) (*model.ExportBatch, error) {
	// Lookup a set of candidate batch IDs.
	var openBatchIDs []int64
	err := func() error { // Use a func to allow defer conn.Release() to work.
		conn, err := db.db.Pool.Acquire(ctx)
		if err != nil {
			return fmt.Errorf("acquiring connection: %w", err)
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
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating rows: %w", err)
			}

			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			openBatchIDs = append(openBatchIDs, id)
		}
		return rows.Err()
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
		err := db.db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {

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

			if status == model.ExportBatchComplete || (expires != nil && status == model.ExportBatchPending && now.Before(*expires)) {
				// Something beat us to this batch, it's no longer available.
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
			return db.LookupExportBatch(ctx, bid)
		}
	}
	// We didn't manage to lease any of the candidates, so return no work to be done (nil).
	return nil, nil
}

// LookupExportBatch returns an ExportBatch for the given batchID.
func (db *ExportDB) LookupExportBatch(ctx context.Context, batchID int64) (*model.ExportBatch, error) {
	conn, err := db.db.Pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring connection: %w", err)
	}
	defer conn.Release()

	return lookupExportBatch(ctx, batchID, conn.QueryRow)
}

type queryRowFn func(ctx context.Context, query string, args ...interface{}) pgx.Row

func lookupExportBatch(ctx context.Context, batchID int64, queryRow queryRowFn) (*model.ExportBatch, error) {
	row := queryRow(ctx, `
		SELECT
			batch_id, config_id, bucket_name, filename_root, start_timestamp, end_timestamp, region, status, lease_expires, signature_info_ids
		FROM
			ExportBatch
		WHERE
			batch_id = $1
		LIMIT 1
		`, batchID)

	var expires *time.Time
	eb := model.ExportBatch{}
	if err := row.Scan(&eb.BatchID, &eb.ConfigID, &eb.BucketName, &eb.FilenameRoot, &eb.StartTimestamp, &eb.EndTimestamp, &eb.Region, &eb.Status, &expires, &eb.SignatureInfoIDs); err != nil {
		if err == pgx.ErrNoRows {
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
	return db.db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		// Update ExportFile for the files created.
		for i, file := range files {
			ef := model.ExportFile{
				BucketName: eb.BucketName,
				Filename:   file,
				BatchID:    eb.BatchID,
				Region:     eb.Region,
				BatchNum:   i + 1,
				BatchSize:  batchSize,
				Status:     model.ExportBatchComplete,
			}
			if err := addExportFile(ctx, tx, &ef); err != nil {
				if err == database.ErrKeyConflict {
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

// LookupExportFiles returns a list of export files for the given ExportConfig exportConfigID.
func (db *ExportDB) LookupExportFiles(ctx context.Context, exportConfigID int64) ([]string, error) {
	conn, err := db.db.Pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring connection: %w", err)
	}
	defer conn.Release()

	rows, err := conn.Query(ctx, `
		SELECT
			ef.filename
		FROM
			ExportFile ef
		INNER JOIN
			ExportBatch eb ON (eb.batch_id = ef.batch_id)
		WHERE
			eb.config_id = $1
		AND
			eb.status = $2
		ORDER BY
			ef.filename
		`, exportConfigID, model.ExportBatchComplete)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var filenames []string
	for rows.Next() {
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		var filename string
		if err := rows.Scan(&filename); err != nil {
			return nil, err
		}
		filenames = append(filenames, filename)
	}
	return filenames, nil
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
	conn, err := db.db.Pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring connection: %w", err)
	}
	defer conn.Release()

	row := conn.QueryRow(ctx, `
		SELECT
			bucket_name, filename, batch_id, region, batch_num, batch_size, status
		FROM
			ExportFile
		WHERE
			filename = $1
		LIMIT 1
		`, filename)

	ef := model.ExportFile{}
	if err := row.Scan(&ef.BucketName, &ef.Filename, &ef.BatchID, &ef.Region, &ef.BatchNum, &ef.BatchSize, &ef.Status); err != nil {
		if err == pgx.ErrNoRows {
			return nil, database.ErrNotFound
		}
		return nil, err
	}
	return &ef, nil
}

// DeleteFilesBefore deletes the export batch files for batches ending before the time passed in.
func (db *ExportDB) DeleteFilesBefore(ctx context.Context, before time.Time, blobstore storage.Blobstore) (int, error) {
	logger := logging.FromContext(ctx)

	// Fetch filenames for  batches where at least one file is not deleted yet.
	var files []joinedExportBatchFile
	err := func() error { // Use a func to allow defer conn.Release() to work.
		conn, err := db.db.Pool.Acquire(ctx)
		if err != nil {
			return fmt.Errorf("acquiring connection: %w", err)
		}
		defer conn.Release()

		q := `
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
				AND eb.status != $2`
		rows, err := conn.Query(ctx, q, before, model.ExportBatchDeleted)
		if err != nil {
			return fmt.Errorf("fetching filenames: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var f joinedExportBatchFile
			if err := rows.Scan(&f.batchID, &f.batchStatus, &f.bucketName, &f.filename, &f.count, &f.fileStatus); err != nil {
				return fmt.Errorf("fetching batch_id: %w", err)
			}
			files = append(files, f)
		}
		return rows.Err()
	}()
	if err != nil {
		return 0, err
	}

	count := 0
	batchFileDeleteCounter := make(map[int64]int)

	for _, f := range files {

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

		err := db.db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
			// Update Status in ExportFile.
			if err := updateExportFileStatus(ctx, tx, f.batchID, f.filename, model.ExportBatchDeleted); err != nil {
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

		logger.Infof("Deleted filename %s", f.filename)
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
			(bucket_name, filename, batch_id, region, batch_num, batch_size, status)
		VALUES
			($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (filename) DO NOTHING
		`, ef.BucketName, ef.Filename, ef.BatchID, ef.Region, ef.BatchNum, ef.BatchSize, ef.Status)
	if err != nil {
		return fmt.Errorf("inserting to ExportFile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return database.ErrKeyConflict
	}
	return nil
}

func updateExportFileStatus(ctx context.Context, tx pgx.Tx, batchID int64, filename, status string) error {
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

func shuffle(vals []int64) []int64 {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ret := make([]int64, len(vals))
	perm := r.Perm(len(vals))
	for i, randIndex := range perm {
		ret[i] = vals[randIndex]
	}
	return ret
}
