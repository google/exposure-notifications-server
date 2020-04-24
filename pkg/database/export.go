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
	"context"
	"database/sql"
	"fmt"
	"time"
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

	_, err = tx.ExecContext(ctx, `
		INSERT INTO ExportBatch
			(start_timestamp, end_timestamp, status)
		VALUES
			($1, $2, $3)
		`, time.Now().UTC().Add(time.Hour*-24), time.Now().UTC(), "OPEN")
	// TODO(guray): lookup end_timestamp of last successful batch, truncate, insert into ExportFile, etc, etc
	if err != nil {
		return fmt.Errorf("inserting export batch: %v", err)
	}

	commit = true
	return nil
}
