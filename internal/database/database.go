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
	"time"

	"github.com/jackc/pgconn"
	pgx "github.com/jackc/pgx/v4"
)

var (
	// ErrNotFound indicates that the requested record was not found in the database.
	ErrNotFound = errors.New("record not found")

	// ErrKeyConflict indicates that there was a key conflict inserting a row.
	ErrKeyConflict = errors.New("key conflict")
)

func (db *DB) NullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// IsSerializationError returns true if the error is a transaction serialization error
// from PG. This should only occur with isolation level of serializable and is
// a retryable condition.
func IsSerializationError(err error) bool {
	if err == nil {
		return false
	}
	// See https://www.postgresql.org/docs/current/errcodes-appendix.html
	if pgErr, ok := err.(*pgconn.PgError); !ok || pgErr.Code == "40001" {
		return true
	}
	return false
}

// InTx runs the given function f within a transaction with isolation level isoLevel.
func (db *DB) InTx(ctx context.Context, isoLevel pgx.TxIsoLevel, f func(tx pgx.Tx) error) error {
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquiring connection: %v", err)
	}
	defer conn.Release()

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: isoLevel})
	if err != nil {
		return fmt.Errorf("starting transaction: %v", err)
	}

	if err := f(tx); err != nil {
		if err1 := tx.Rollback(ctx); err1 != nil {
			return fmt.Errorf("rolling back transaction: %v (original error: %v)", err1, err)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %v", err)
	}
	return nil
}
