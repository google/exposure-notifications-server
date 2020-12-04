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

// Package database is a database interface for mirror settings.
package database

import (
	"context"
	"fmt"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/mirror/model"
	"github.com/jackc/pgx/v4"
)

type MirrorDB struct {
	db *database.DB
}

func New(db *database.DB) *MirrorDB {
	return &MirrorDB{
		db: db,
	}
}

func (db *MirrorDB) AddMirror(ctx context.Context, m *model.Mirror) error {
	return db.db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO
				Mirror (index_file, export_root, cloud_storage_bucket, filename_root, filename_rewrite)
			VALUES
				($1, $2, $3, $4, $5)
			RETURNING id
		`, m.IndexFile, m.ExportRoot, m.CloudStorageBucket, m.FilenameRoot, m.FilenameRewrite)

		if err := row.Scan(&m.ID); err != nil {
			return fmt.Errorf("fetching mirror.ID: %w", err)
		}
		return nil
	})
}

// UpdateMirror updates the given mirror struct in the database. It must already
// exist in the database, keyed off of ID.
func (db *MirrorDB) UpdateMirror(ctx context.Context, m *model.Mirror) error {
	return db.db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE
				Mirror
			SET
				index_file = $2,
				export_root = $3,
				cloud_storage_bucket = $4,
				filename_root = $5,
				filename_rewrite = $6
			WHERE id = $1
		`, m.ID, m.IndexFile, m.ExportRoot, m.CloudStorageBucket, m.FilenameRoot, m.FilenameRewrite)
		if err != nil {
			return fmt.Errorf("failed to update mirror: %w", err)
		}

		switch v := result.RowsAffected(); v {
		case 0:
			return fmt.Errorf("no rows were updated (does the record exist?)")
		case 1:
			return nil
		default:
			return fmt.Errorf("only 1 row should have been updated, got %d", v)
		}
	})
}

func (db *MirrorDB) DeleteMirror(ctx context.Context, m *model.Mirror) error {
	return db.db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			DELETE FROM
				MirrorFile
			WHERE
				mirror_id = $1
			`, m.ID)
		if err != nil {
			return fmt.Errorf("failed to delete mirror files: %w", err)
		}

		_, err = tx.Exec(ctx, `
			DELETE FROM
				Mirror
			WHERE
				id = $1
			`, m.ID)
		if err != nil {
			return fmt.Errorf("failed to delete mirror config: %w", err)
		}

		return nil
	})
}

// GetMirror retruns the mirror with the given ID.
func (db *MirrorDB) GetMirror(ctx context.Context, id int64) (*model.Mirror, error) {
	var mirror *model.Mirror

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT
				id, index_file, export_root, cloud_storage_bucket, filename_root, filename_rewrite
			FROM
				mirror
			WHERE
				id = $1
		`, id)

		var err error
		mirror, err = scanOneMirror(row)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to get mirror %d: %w", id, err)
	}

	return mirror, nil
}

// Mirrors returns the list of mirrors for the database, ordered by id.
func (db *MirrorDB) Mirrors(ctx context.Context) ([]*model.Mirror, error) {
	var mirrors []*model.Mirror

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				id, index_file, export_root, cloud_storage_bucket, filename_root, filename_rewrite
			FROM
				mirror
			ORDER BY id
		`)
		if err != nil {
			return fmt.Errorf("failed to list: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}

			m, err := scanOneMirror(rows)
			if err != nil {
				return err
			}
			mirrors = append(mirrors, m)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("listing mirror configs: %w", err)
	}

	return mirrors, nil
}

// scanOneMirror scans a single pgx row into a mirror model.
func scanOneMirror(row pgx.Row) (*model.Mirror, error) {
	var m model.Mirror
	if err := row.Scan(&m.ID, &m.IndexFile, &m.ExportRoot, &m.CloudStorageBucket, &m.FilenameRoot, &m.FilenameRewrite); err != nil {
		return nil, fmt.Errorf("failed to scan mirror row: %w", err)
	}
	return &m, nil
}

type SyncFile struct {
	// RemoteFile is the ONLY the final filename (last part after the slash) with
	// extension. It does not include the URL, protocol or root information as
	// that is built from the parent mirror record.
	RemoteFile string

	// LocalFile is blank unless a rewrite rule was provided. It is also just the
	// filename (no URL or protocol information).
	LocalFile string
}

// SaveFiles makes the list of filenames passed in the only files that are saved on that mirrorID.
// filenames is a map of the upstream->local filenames. They may be the same.
func (db *MirrorDB) SaveFiles(ctx context.Context, mirrorID int64, filenames []*SyncFile) error {
	const deleteName = "delete mirror file"
	const insertName = "insert mirror file"

	wantFiles := make(map[string]string, len(filenames))
	for _, sf := range filenames {
		wantFiles[sf.RemoteFile] = sf.LocalFile
	}

	return db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		// Read files selects all of the existing known files FOR UPDATE.
		knownFiles, err := readFiles(ctx, tx, mirrorID)
		if err != nil {
			return err
		}

		toDelete := make([]*model.MirrorFile, 0)
		// if any filenames were read that aren't in the 'filenames' list, add them to the toDelete
		for _, mirrorFile := range knownFiles {
			if _, ok := wantFiles[mirrorFile.Filename]; ok {
				delete(wantFiles, mirrorFile.Filename)
			} else {
				toDelete = append(toDelete, mirrorFile)
			}
		}

		// if any filenames need removing, delete them.
		// toDelete contains items in 'knownFiles' that weren't in 'filenames'
		if len(toDelete) > 0 {
			if _, err := tx.Prepare(ctx, deleteName, `
				DELETE FROM
						MirrorFile
				WHERE
					mirror_id = $1 AND filename = $2
				`); err != nil {
				return fmt.Errorf("failed to prepare delete DB statement: %w", err)
			}

			for _, mf := range toDelete {
				if result, err := tx.Exec(ctx, deleteName, mf.MirrorID, mf.Filename); err != nil {
					return fmt.Errorf("failed to delete mirrofile: %w", err)
				} else if result.RowsAffected() != 1 {
					return fmt.Errorf("delete of locked row failed")
				}
			}
		}

		// Create files if they still need to be created.
		// wantFiles contains items from 'filenames' that weren't in 'knownFiles'
		if len(wantFiles) > 0 {
			if _, err := tx.Prepare(ctx, insertName, `
				INSERT INTO
					MirrorFile (mirror_id, filename, local_filename)
				VALUES
					($1, $2, $3)
				ON CONFLICT (mirror_id, filename) DO NOTHING
			`); err != nil {
				return fmt.Errorf("failed to prepare insert statement: %w", err)
			}

			for fName, rewrittenFilename := range wantFiles {
				var localFilename *string
				if fName != rewrittenFilename {
					localFilename = &rewrittenFilename
				}
				if _, err := tx.Exec(ctx, insertName, mirrorID, fName, localFilename); err != nil {
					return fmt.Errorf("failed to insert mirrorfile: %w", err)
				}
			}
		}

		return nil
	})
}

func (db *MirrorDB) ListFiles(ctx context.Context, mirrorID int64) ([]*model.MirrorFile, error) {
	var mirrorFiles []*model.MirrorFile

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		var err error
		mirrorFiles, err = readFiles(ctx, tx, mirrorID)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return mirrorFiles, nil
}

func readFiles(ctx context.Context, tx pgx.Tx, mirrorID int64) ([]*model.MirrorFile, error) {
	var mirrorFiles []*model.MirrorFile
	rows, err := tx.Query(ctx, `
			SELECT
				mirror_id, filename, local_filename
			FROM
				MirrorFile
			WHERE
				mirror_id = $1
			FOR UPDATE
		`, mirrorID)
	if err != nil {
		return nil, fmt.Errorf("failed to list: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("failed to iterate: %w", err)
		}

		var f model.MirrorFile
		if err := rows.Scan(&f.MirrorID, &f.Filename, &f.LocalFilename); err != nil {
			return nil, fmt.Errorf("reading row: %w", err)
		}
		mirrorFiles = append(mirrorFiles, &f)
	}

	return mirrorFiles, nil
}
