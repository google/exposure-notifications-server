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
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	statusOpen = "OPEN"
	statusDone = "DONE"
)

func AddNewBatch(ctx context.Context) (err error) {
	conn, err := Connection(ctx)
	if err != nil {
		return fmt.Errorf("unable to obtain database connection: %v", err)
	}
	logger := logging.FromContext(ctx)

	commit := false
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("starting transaction: %v", err)
	}
	defer func() {
		if commit {
			if err1 := tx.Commit(); err1 != nil {
				err = fmt.Errorf("failed to commit: %v", err1)
			}
		} else {
			if err1 := tx.Rollback(); err1 != nil {
				err = fmt.Errorf("failed to rollback: %v", err1)
			} else {
				logger.Debugf("Rolling back.")
			}
		}
	}()
	eb, err1 := calculateNextBatch(ctx, tx)
	if err1 != nil {
		return fmt.Errorf("calculating batch: %v", err1)
	}
	bId, err2 := insertBatch(ctx, tx, eb)
	if err2 != nil {
		return fmt.Errorf("inserting batch: %v", err2)
	}
	logger.Infof("Inserted batch id %v", bId)

	// TODO(guray): insert into ExportFile with that batch, query for exposure key counts, ...

	commit = true
	return nil
}

func calculateNextBatch(ctx context.Context, tx *sql.Tx) (model.ExportBatch, error) {
	// TODO(guray): lookup end_timestamp of last successful batch, truncate, etc
	return model.ExportBatch{
		StartTimestamp: time.Now().UTC().Add(time.Hour * -24),
		EndTimestamp:   time.Now().UTC(),
		Status:         statusOpen,
	}, nil
}

func insertBatch(ctx context.Context, tx *sql.Tx, b model.ExportBatch) (int64, error) {
	// Postgres sql driver doesn't support LastInsertedId, so using "RETURNING"
	// and scanning the query result
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO ExportBatch
			(start_timestamp, end_timestamp, status)
		VALUES
			($1, $2, $3)
		RETURNING batch_id
		`)
	if err != nil {
		return -1, err
	}
	defer stmt.Close()
	var id int64
	err1 := stmt.QueryRowContext(ctx, b.StartTimestamp, b.EndTimestamp, b.Status).Scan(&id)
	if err1 != nil {
		return -1, err1
	}
	return id, nil
}
