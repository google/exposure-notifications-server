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

	"github.com/google/exposure-notifications-server/internal/logging"
	pgx "github.com/jackc/pgx/v4"
)

var (
	// ErrAlreadyLocked is returned if the lock is already in use.
	ErrAlreadyLocked = errors.New("lock already in use")

	// thePast is a date sufficiently in the past to allow safer comparison to "zero date" from Postgres.
	thePast = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
)

// UnlockFn can be deferred to release a lock.
type UnlockFn func() error

// Lock acquires lock with given name that times out after ttl. Returns an UnlockFn that can be used to unlock the lock. ErrAlreadyLocked will be returned if there is already a lock in use.
func (db *DB) Lock(ctx context.Context, lockID string, ttl time.Duration) (UnlockFn, error) {
	var expires time.Time
	err := db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT AcquireLock($1, $2)
		`, lockID, int(ttl.Seconds()))
		if err := row.Scan(&expires); err != nil {
			return err
		}
		if expires.Before(thePast) {
			return ErrAlreadyLocked
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	logging.FromContext(ctx).Debugf("Acquired lock %q", lockID)
	return makeUnlockFn(ctx, db, lockID, expires), nil
}

func makeUnlockFn(ctx context.Context, db *DB, lockID string, expires time.Time) UnlockFn {
	return func() error {
		return db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
			row := tx.QueryRow(ctx, `
				SELECT ReleaseLock($1, $2)
			`, lockID, expires)
			var released bool
			if err := row.Scan(&released); err != nil {
				return err
			}
			if !released {
				return fmt.Errorf("cannot delete lock %q that no longer belongs to you; it likely expired and was taken by another process", lockID)
			}
			logging.FromContext(ctx).Debugf("Released lock %q", lockID)
			return nil
		})
	}
}
