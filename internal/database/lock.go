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
	"sort"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
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

// MultiLock obtains multiple locks in a single transaction. Either all locks are obtained, or
// the transaction is rolled back.
// The lockIDs are sorted by normal ascending string sort order before obtaining the locks.
func (db *DB) MultiLock(ctx context.Context, lockIDs []string, ttl time.Duration) (UnlockFn, error) {
	if len(lockIDs) == 0 {
		return nil, fmt.Errorf("no lockIDs presented")
	}

	lockOrder := make([]string, len(lockIDs))
	// Make a copy of the slice so that we have a stable slice in the unlcok function.
	copy(lockOrder, lockIDs)
	sort.Strings(lockOrder)

	var expires time.Time
	err := db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		for _, lockID := range lockOrder {
			row := tx.QueryRow(ctx, `
			SELECT AcquireLock($1, $2)
		`, lockID, int(ttl.Seconds()))
			if err := row.Scan(&expires); err != nil {
				return err
			}
			if expires.Before(thePast) {
				return ErrAlreadyLocked
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	logging.FromContext(ctx).Debugf("Acquired locks %v", lockOrder)
	return makeMultiUnlockFn(ctx, db, lockOrder, expires), nil
}

func makeMultiUnlockFn(ctx context.Context, db *DB, lockIDs []string, expires time.Time) UnlockFn {
	return func() error {
		return db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
			for i := len(lockIDs) - 1; i >= 0; i-- {
				lockID := lockIDs[i]
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
			}
			return nil
		})
	}
}

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
