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
	"fmt"
	"time"

	pgx "github.com/jackc/pgx/v4"
)

const (
	statusOpen = "OPEN"
	statusDone = "DONE"
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
