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

// Package database is a database interface for export importing.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/exposure-notifications-server/internal/exportimport/model"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/jackc/pgx/v4"
)

// ExportImportDB contains database methods for managing with export-import configs.
type ExportImportDB struct {
	db *database.DB
}

func New(db *database.DB) *ExportImportDB {
	return &ExportImportDB{
		db: db,
	}
}

// GetConfig gets the configuration for the given id.
func (db *ExportImportDB) GetConfig(ctx context.Context, id int64) (*model.ExportImport, error) {
	var config *model.ExportImport

	if err := db.db.InTx(ctx, pgx.RepeatableRead, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT
				id, index_file, export_root, region, traveler, from_timestamp, thru_timestamp
			FROM
				exportimport
			WHERE
				id = $1`, id)

		var err error
		config, err = scanOneConfig(row)
		if err != nil {
			return fmt.Errorf("failed to scan: %w", err)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to lookup export importer config: %w", err)
	}

	return config, nil
}

// ListConfigs lists all configs (active and inactive). This is a utility method for the
// admin console.
func (db *ExportImportDB) ListConfigs(ctx context.Context) ([]*model.ExportImport, error) {
	var configs []*model.ExportImport

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				id, index_file, export_root, region, traveler, from_timestamp, thru_timestamp
			FROM
				exportimport
			ORDER BY id ASC
		`)
		if err != nil {
			return fmt.Errorf("failed to list: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}

			cfg, err := scanOneConfig(rows)
			if err != nil {
				return err
			}
			configs = append(configs, cfg)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to list export importer configs: %w", err)
	}

	return configs, nil
}

// ActiveConfigs returns the active export import configurations.
func (db *ExportImportDB) ActiveConfigs(ctx context.Context) ([]*model.ExportImport, error) {
	var configs []*model.ExportImport

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				id, index_file, export_root, region, traveler, from_timestamp, thru_timestamp
			FROM
				exportimport
			WHERE
				from_timestamp <= $1
			AND
				(thru_timestamp IS NULL OR thru_timestamp >= $1)
			ORDER BY id ASC
		`, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("failed to list: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}

			cfg, err := scanOneConfig(rows)
			if err != nil {
				return err
			}
			configs = append(configs, cfg)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("listing active exportimport configs: %w", err)
	}

	return configs, nil
}

func scanOneConfig(row pgx.Row) (*model.ExportImport, error) {
	var (
		m    model.ExportImport
		thru *time.Time
	)

	if err := row.Scan(&m.ID, &m.IndexFile, &m.ExportRoot, &m.Region, &m.Traveler, &m.From, &thru); err != nil {
		return nil, err
	}
	if thru != nil {
		m.Thru = thru
	}

	return &m, nil
}

// AddConfig saves a new ExportImport configuration.
func (db *ExportImportDB) AddConfig(ctx context.Context, ei *model.ExportImport) error {
	if err := ei.Validate(); err != nil {
		return err
	}

	return db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO
			ExportImport
				(index_file, export_root, region, traveler, from_timestamp, thru_timestamp)
			VALUES
				($1, $2, $3, $4, $5, $6)
			RETURNING id
		`, ei.IndexFile, ei.ExportRoot, ei.Region, ei.Traveler, ei.From, ei.Thru)

		if err := row.Scan(&ei.ID); err != nil {
			return fmt.Errorf("fetching exportimport.ID: %w", err)
		}
		return nil
	})
}

// UpdateConfig updates an existing ExportImporter.
func (db *ExportImportDB) UpdateConfig(ctx context.Context, c *model.ExportImport) error {
	if err := c.Validate(); err != nil {
		return err
	}

	from := database.NullableTime(c.From)
	return db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE
				ExportImport
			SET
				index_file = $1, export_root = $2, region = $3, traveler = $4, from_timestamp = $5, thru_timestamp = $6
			WHERE id = $7
		`, c.IndexFile, c.ExportRoot, c.Region, c.Traveler, from, c.Thru, c.ID)
		if err != nil {
			return fmt.Errorf("failed to update export importer config: %w", err)
		}

		switch v := result.RowsAffected(); v {
		case 0:
			return fmt.Errorf("no rows updated (does the record exist?)")
		case 1:
			return nil
		default:
			return fmt.Errorf("only 1 row should have been updated, but %d were", v)
		}
	})
}

func (db *ExportImportDB) ExpireImportFilePublicKey(ctx context.Context, ifpk *model.ImportFilePublicKey) error {
	return db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		now := time.Now().UTC()
		if thru := ifpk.Thru; thru != nil && now.After(*thru) {
			return fmt.Errorf("importfilepublic key is already expired")
		}
		ifpk.Thru = &now
		result, err := tx.Exec(ctx, `
			UPDATE ImportFilePublicKey
			SET
				thru_timestamp = $1
			WHERE
				export_import_id = $2 AND
				key_id = $3 AND
				key_version = $4
			`, ifpk.Thru, ifpk.ExportImportID, ifpk.KeyID, ifpk.KeyVersion)
		if err != nil {
			return fmt.Errorf("expiring importfilepublickey: %w", err)
		}
		if result.RowsAffected() != 1 {
			return fmt.Errorf("importfilepublickey not found while expiring")
		}
		return nil
	})
}

func (db *ExportImportDB) SavePublicKeyTimestamps(ctx context.Context, ifpk *model.ImportFilePublicKey) error {
	return db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE ImportFilePublicKey
			SET
				from_timestamp = $1,
				thru_timestamp = $2
			WHERE
				export_import_id = $3 AND
				key_id = $4 AND
				key_version = $5
			`, ifpk.From, ifpk.Thru, ifpk.ExportImportID, ifpk.KeyID, ifpk.KeyVersion)
		if err != nil {
			return fmt.Errorf("changing times importfilepublickey: %w", err)
		}
		if result.RowsAffected() != 1 {
			return fmt.Errorf("importfilepublickey not found")
		}
		return nil
	})
}

func (db *ExportImportDB) AddImportFilePublicKey(ctx context.Context, ifpk *model.ImportFilePublicKey) error {
	return db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			INSERT INTO
				ImportFilePublicKey
				(export_import_id, key_id, key_version, public_key, from_timestamp, thru_timestamp)
			VALUES
				($1, $2, $3, $4, $5, $6)
			`, ifpk.ExportImportID, ifpk.KeyID, ifpk.KeyVersion, ifpk.PublicKeyPEM, ifpk.From, ifpk.Thru)
		if err != nil {
			return fmt.Errorf("inserting importfilepublickey: %w", err)
		}
		if result.RowsAffected() != 1 {
			return fmt.Errorf("no rows inserted")
		}
		return nil
	})
}

func (db *ExportImportDB) AllPublicKeys(ctx context.Context, ei *model.ExportImport) ([]*model.ImportFilePublicKey, error) {
	var publicKeys []*model.ImportFilePublicKey

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				export_import_id, key_id, key_version, public_key, from_timestamp, thru_timestamp
			FROM
				ImportFilePublicKey
			WHERE
				export_import_id = $1
			ORDER BY
				from_timestamp DESC
		`, ei.ID)
		if err != nil {
			return fmt.Errorf("failed to list: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}

			publicKey, err := scanOnePublicKey(rows)
			if err != nil {
				return err
			}
			publicKeys = append(publicKeys, publicKey)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return publicKeys, nil
}

func (db *ExportImportDB) AllowedKeys(ctx context.Context, ei *model.ExportImport) ([]*model.ImportFilePublicKey, error) {
	var publicKeys []*model.ImportFilePublicKey

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				export_import_id, key_id, key_version, public_key, from_timestamp, thru_timestamp
			FROM
				ImportFilePublicKey
			WHERE
				export_import_id = $1
			AND
				(thru_timestamp IS NULL OR thru_timestamp > $2)
			ORDER BY
				from_timestamp DESC
		`, ei.ID, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("failed to list: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}

			publicKey, err := scanOnePublicKey(rows)
			if err != nil {
				return err
			}
			publicKeys = append(publicKeys, publicKey)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return publicKeys, nil
}

func scanOnePublicKey(row pgx.Row) (*model.ImportFilePublicKey, error) {
	var (
		m    model.ImportFilePublicKey
		thru *time.Time
	)

	if err := row.Scan(&m.ExportImportID, &m.KeyID, &m.KeyVersion, &m.PublicKeyPEM, &m.From, &thru); err != nil {
		return nil, err
	}
	if thru != nil {
		m.Thru = thru
	}

	return &m, nil
}

func prepareInsertImportFile(ctx context.Context, tx pgx.Tx) (string, error) {
	const stmtName = "insert importfile"
	_, err := tx.Prepare(ctx, stmtName, `
		INSERT INTO
			ImportFile
				(export_import_id, zip_filename, discovered_at)
		VALUES
			($1, $2, $3)
		ON CONFLICT DO NOTHING
		RETURNING id
	`)
	return stmtName, err
}

// CreateNewFilesAndFailOld creates all the specified files named, returning
// the number of created files, and the number moved to an failed state.
func (db *ExportImportDB) CreateNewFilesAndFailOld(ctx context.Context, ei *model.ExportImport, filenames []string) (int, int, error) {
	logger := logging.FromContext(ctx)
	insertedFiles, failedFiles := 0, 0

	now := time.Now().UTC()
	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		// read existing filenames
		rows, err := tx.Query(ctx, `
			SELECT
				zip_filename
			FROM
				ImportFile
			WHERE export_import_id=$1 AND zip_filename = ANY($2)
		`, ei.ID, filenames)
		if err != nil {
			return fmt.Errorf("failed to read existing filenames: %w", err)
		}
		defer rows.Close()

		// put existing ones into a map
		existing := make(map[string]struct{})
		for rows.Next() {
			var fname string
			if err := rows.Scan(&fname); err != nil {
				return fmt.Errorf("failed to read existing filename: %w", err)
			}
			existing[fname] = struct{}{}
		}

		// Go through incoming list and insert a new entry for each filename we haven't seen before.
		insertStmt, err := prepareInsertImportFile(ctx, tx)
		if err != nil {
			return fmt.Errorf("preparing insert statement: %w", err)
		}

		for _, fname := range filenames {
			if _, ok := existing[fname]; ok {
				// we've already scheduled this file previously, skip.
				continue
			}

			result, err := tx.Exec(ctx, insertStmt, ei.ID, fname, now)
			if err != nil {
				return fmt.Errorf("error inserting filename: %v, %w", fname, err)
			}
			if result.RowsAffected() != 1 {
				logger.Warnw("attempted to insert duplicate file", "exportImportID", ei.ID)
				continue
			}
			logger.Debugw("scheduled new export file for importing", "exportImportID", ei.ID, "filename", fname)
			insertedFiles++
		}

		// Close any files that aren't in the list that are retrying.
		failed, err := tx.Exec(ctx, `
				UPDATE
					ImportFile
				SET
					status = $1
				WHERE export_import_id=$2 AND NOT zip_filename = ANY($3) AND status != $4
			`, model.ImportFileFailed, ei.ID, filenames, model.ImportFileComplete)
		if err != nil {
			return fmt.Errorf("failed to update retries: %w", err)
		}
		failedFiles = int(failed.RowsAffected())

		return nil
	}); err != nil {
		return 0, 0, fmt.Errorf("creating import files: %w", err)
	}

	return insertedFiles, failedFiles, nil
}

func (db *ExportImportDB) CompleteImportFile(ctx context.Context, ef *model.ImportFile, status string) error {
	now := time.Now().UTC()
	return db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				status
			FROM
				ImportFile
			WHERE
				id = $1
			FOR UPDATE
		`, ef.ID)
		if err != nil {
			return err
		}
		if !rows.Next() {
			rows.Close()
			return fmt.Errorf("record not found")
		}
		var curStatus string
		if err := rows.Scan(&curStatus); err != nil {
			rows.Close()
			return fmt.Errorf("unable to read row: %w", err)
		}

		if curStatus != model.ImportFilePending {
			rows.Close()
			return fmt.Errorf("cannot complete from status: %v", curStatus)
		}
		rows.Close()

		ef.Status = status
		ef.ProcessedAt = &now
		result, err := tx.Exec(ctx, `
			UPDATE
				ImportFile
			SET
				status=$1, processed_at=$2, retries=$3
			WHERE
				id=$4
			`, ef.Status, ef.ProcessedAt, ef.Retries, ef.ID)
		if err != nil {
			return fmt.Errorf("unable to mark complete: %w", err)
		}
		if result.RowsAffected() != 1 {
			return fmt.Errorf("marking complete did not change any rows")
		}
		return nil
	})
}

func (db *ExportImportDB) LeaseImportFile(ctx context.Context, lockDuration time.Duration, ef *model.ImportFile) error {
	now := time.Now().UTC().Truncate(time.Second)
	return db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				status, processed_at
			FROM
				ImportFile
			WHERE
				id = $1
			FOR UPDATE
		`, ef.ID)
		if err != nil {
			return err
		}
		var curStatus string
		var processedAt *time.Time
		if !rows.Next() {
			rows.Close()
			return fmt.Errorf("record not found")
		}
		if err := rows.Scan(&curStatus, &processedAt); err != nil {
			rows.Close()
			return fmt.Errorf("unable to read row: %w", err)
		}
		rows.Close()

		// A file that is open or was put into pending more than lockDuration ago can be locked.
		okToLock := curStatus == model.ImportFileOpen ||
			(curStatus == model.ImportFilePending &&
				(processedAt == nil || now.Sub(*processedAt) > lockDuration))
		if !okToLock {
			return fmt.Errorf("file not elliglble for processing")
		}

		ef.Status = model.ImportFilePending
		ef.ProcessedAt = &now
		result, err := tx.Exec(ctx, `
			UPDATE
				ImportFile
			SET
				status=$1, processed_at=$2
			WHERE
				id=$3
			`, ef.Status, ef.ProcessedAt, ef.ID)
		if err != nil {
			return fmt.Errorf("unable to lock file: %w", err)
		}
		if result.RowsAffected() != 1 {
			return fmt.Errorf("row locking didn't update any rows")
		}
		return nil
	})
}

func (db *ExportImportDB) GetOpenImportFiles(ctx context.Context, lockDuration, retryRate time.Duration, ei *model.ExportImport) ([]*model.ImportFile, error) {
	var importFiles []*model.ImportFile

	lockOverrideTime := time.Now().UTC().Add(-1 * lockDuration)
	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				id, zip_filename, discovered_at, processed_at, status, retries
			FROM
				ImportFile
			WHERE
				export_import_id = $1 AND (status = $2 OR (status = $3 AND processed_at < $4))
			ORDER BY
				id ASC
		`, ei.ID, model.ImportFileOpen, model.ImportFilePending, lockOverrideTime)
		if err != nil {
			return fmt.Errorf("failed to list: %w", err)
		}

		defer rows.Close()
		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}

			file := model.ImportFile{
				ExportImportID: ei.ID,
			}
			if err := rows.Scan(&file.ID, &file.ZipFilename, &file.DiscoveredAt, &file.ProcessedAt, &file.Status, &file.Retries); err != nil {
				return fmt.Errorf("failed to scan rows: %w", err)
			}

			// Do some backoff on the retrying ImportFiles.
			if file.ShouldTry(retryRate) {
				importFiles = append(importFiles, &file)
			} else {
				fmt.Println("SKIP:", file)
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("unable to read open import files: %w", err)
	}

	return importFiles, nil
}

// GetAllImportFiles returns all input files for a config, regardless of their state.
// This function is used for testing.
func (db *ExportImportDB) GetAllImportFiles(ctx context.Context, lockDuration time.Duration, ei *model.ExportImport) ([]*model.ImportFile, error) {
	var importFiles []*model.ImportFile

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				id, zip_filename, discovered_at, status, retries
			FROM
				ImportFile
			WHERE
				export_import_id = $1
			ORDER BY
				id ASC
		`, ei.ID)
		if err != nil {
			return fmt.Errorf("failed to list: %w", err)
		}

		defer rows.Close()
		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}

			file := model.ImportFile{
				ExportImportID: ei.ID,
			}
			if err := rows.Scan(&file.ID, &file.ZipFilename, &file.DiscoveredAt, &file.Status, &file.Retries); err != nil {
				return fmt.Errorf("failed to scan rows: %w", err)
			}

			importFiles = append(importFiles, &file)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("unable to read open import files: %w", err)
	}

	return importFiles, nil
}
