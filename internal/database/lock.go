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

	"github.com/google/exposure-notifications-server/internal/model"

	pgx "github.com/jackc/pgx/v4"
)

var (
	// ErrAlreadyLocked is returned if the lock is already in use.
	ErrAlreadyLocked = errors.New("lock already in use")
)

// UnlockFn can be deferred to release a lock.
type UnlockFn func() error

// Lock acquires lock with given name that times out after ttl. Returns an UnlockFn that can be used to unlock the lock. ErrAlreadyLocked will be returned if there is already a lock in use.
func (db *DB) Lock(ctx context.Context, lockID string, ttl time.Duration) (UnlockFn, error) {
	err := db.inTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {

		// Lookup existing lock, if any.
		row := tx.QueryRow(ctx, `
			SELECT
				lock_id, expires 
			FROM
				Lock 
			WHERE
				lock_id=$1
		`, lockID)

		existing := true
		var l model.Lock
		if err := row.Scan(&l.LockID, &l.Expires); err != nil {
			if err == pgx.ErrNoRows {
				existing = false
			} else {
				return fmt.Errorf("scanning results: %v", err)
			}
		}

		expiry := time.Now().UTC().Add(ttl)
		if existing {
			// If expired, update lock and return true.
			if time.Now().UTC().After(l.Expires) {
				_, err := tx.Exec(ctx, `
					UPDATE
						Lock
					SET
						expires=$1
					WHERE
						lock_id=$2
				`, expiry, lockID)
				if err != nil {
					return fmt.Errorf("updating expired lock: %v", err)
				}
				return nil
			}
			return ErrAlreadyLocked
		}

		// Insert a new lock.
		_, err := tx.Exec(ctx, `
			INSERT INTO
				Lock
				(lock_id, expires)
			VALUES
				($1, $2)
		`, lockID, expiry)
		if err != nil {
			return fmt.Errorf("inserting new lock: %v", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return makeUnlockFn(ctx, db, lockID), nil
}

func makeUnlockFn(ctx context.Context, db *DB, lockID string) UnlockFn {
	return func() error {
		return db.inTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				DELETE FROM
					Lock
				WHERE
					lock_id=$1
		`, lockID)
			if err != nil {
				return fmt.Errorf("deleting lock: %v", err)
			}
			return nil
		})
	}
}
