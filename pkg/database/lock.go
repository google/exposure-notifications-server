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
	"errors"
	"fmt"
	"time"
)

var (
	// ErrAlreadyLocked is returned if the lock is already in use.
	ErrAlreadyLocked = errors.New("lock already in use")
)

// UnlockFn can be deferred to release a lock.
type UnlockFn func() error

// Lock acquires lock with given name that times out after ttl. Returns an UnlockFn that can be used to unlock the lock. ErrAlreadyLocked will be returned if there is already a lock in use.
func Lock(ctx context.Context, lockID string, ttl time.Duration) (unlockFn UnlockFn, err error) {
	conn, err := Connection(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to obtain database connection: %v", err)
	}
	logger := logging.FromContext(ctx)

	now := time.Now().UTC()
	expiry := now.Add(ttl)

	commit := false
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %v", err)
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

	// Lookup existing lock, if any.
	row := tx.QueryRowContext(ctx, `
		SELECT
			lock_id, expires 
		FROM Lock 
		WHERE
			lock_id=$1
		`, lockID)
	if err != nil {
		return nil, fmt.Errorf("getting lock %q: %v", lockID, err)
	}

	existing := true
	var l model.Lock
	if err := row.Scan(&l.LockID, &l.Expires); err != nil {
		if err == sql.ErrNoRows {
			existing = false
		} else {
			return nil, fmt.Errorf("scanning results: %v", err)
		}
	}

	if existing {
		// If expired, update lock and return true
		if time.Now().UTC().After(l.Expires) {
			_, err := tx.ExecContext(ctx, `
				UPDATE Lock
				SET
					expires=$1
				WHERE
					lock_id=$2
				`, expiry, lockID)
			if err != nil {
				return nil, fmt.Errorf("updating expired lock: %v", err)
			}
			commit = true
			return buildUnlockFn(ctx, lockID), nil
		}
		return nil, ErrAlreadyLocked
	}

	// Insert a new lock.
	_, err = tx.ExecContext(ctx, `
		INSERT INTO Lock
			(lock_id, expires)
		VALUES
			($1, $2)
		`, lockID, expiry)
	if err != nil {
		return nil, fmt.Errorf("inserting new lock: %v", err)
	}
	commit = true
	return buildUnlockFn(ctx, lockID), nil
}

func buildUnlockFn(ctx context.Context, lockID string) UnlockFn {
	return func() (err error) {
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
			DELETE FROM Lock
			WHERE
				lock_id=$1
		`, lockID)
		if err != nil {
			return fmt.Errorf("deleting lock: %v", err)
		}
		commit = true
		return nil
	}
}
