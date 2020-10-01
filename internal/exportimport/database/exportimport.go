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

// Package database is a database interface for export importing.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/exportimport/model"
	"github.com/jackc/pgx/v4"
)

type ExportImportDB struct {
	db *database.DB
}

func New(db *database.DB) *ExportImportDB {
	return &ExportImportDB{
		db: db,
	}
}

func (db *ExportImportDB) ActiveConfigs(ctx context.Context) ([]*model.ExportImport, error) {
	var configs []*model.ExportImport

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				id, index_file, export_root, region, from_timestamp, thru_timestamp
			FROM
				exportimport
			WHERE
				from_timestamp <= $1
			AND
				(thru_timestamp IS NULL OR thru_timestamp >= $1)
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

	if err := row.Scan(&m.ID, &m.IndexFile, &m.ExportRoot, &m.Region, &m.From, &thru); err != nil {
		return nil, err
	}
	if thru != nil {
		m.Thru = thru
	}

	return &m, nil
}

func (db *ExportImportDB) AddConfig(ctx context.Context, ei *model.ExportImport) error {
	if err := ei.Validate(); err != nil {
		return err
	}

	return db.db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO
			ExportImport
				(index_file, export_root, region, from_timestamp, thru_timestamp)
			VALUES
				($1, $2, $3, $4, $5)
			RETURNING id
		`, ei.IndexFile, ei.ExportRoot, ei.Region, ei.From, ei.Thru)

		if err := row.Scan(&ei.ID); err != nil {
			return fmt.Errorf("fetching exportimport.ID: %w", err)
		}
		return nil
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
		RETURNING id
	`)
	return stmtName, err
}

func (db *ExportImportDB) CreateFiles(ctx context.Context, ei *model.ExportImport, filenames []string) (int, error) {
	insertedFiles := 0

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
			return fmt.Errorf("preparing insert statement: %v", err)
		}

		for _, fname := range filenames {
			result, err := tx.Exec(ctx, insertStmt, ei.ID, fname, now)
			if err != nil {
				return fmt.Errorf("error inserting filename: %v, %w", fname, err)
			}
			if result.RowsAffected() != 1 {
				return fmt.Errorf("filename isnert failed: %v", fname)
			}
			insertedFiles++
		}
		return nil
	}); err != nil {
		return 0, fmt.Errorf("creating import files: %w", err)
	}

	return insertedFiles, nil
}

func (db *ExportImportDB) CompleteImportFile(ctx context.Context, ef *model.ImportFile) error {
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

		ef.Status = model.ImportFileComplete
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

func (db *ExportImportDB) GetOpenImportFiles(ctx context.Context, lockDuration time.Duration, ei *model.ExportImport) ([]*model.ImportFile, error) {
	var importFiles []*model.ImportFile

	lockOverrideTime := time.Now().UTC().Add(-1 * lockDuration)
	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				id, zip_filename, discovered_at, status
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
			if err := rows.Scan(&file.ID, &file.ZipFilename, &file.DiscoveredAt, &file.Status); err != nil {
				return err
			}
			importFiles = append(importFiles, &file)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("unable to read open import files: %w", err)
	}

	return importFiles, nil
}
