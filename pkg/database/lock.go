// Copyright 2020 the Exposure Notifications Server authors
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
	logger := logging.FromContext(ctx).Named("database.MultiLock")

	if len(lockIDs) == 0 {
		return nil, fmt.Errorf("no lockIDs")
	}

	lockOrder := make([]string, len(lockIDs))
	// Make a copy of the slice so that we have a stable slice in the unlcok function.
	copy(lockOrder, lockIDs)
	sort.Strings(lockOrder)

	var expires time.Time
	if err := db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		for _, lockID := range lockOrder {
			row := tx.QueryRow(ctx, `SELECT AcquireLock($1, $2)`, lockID, int32(ttl.Seconds()))
			if err := row.Scan(&expires); err != nil {
				return fmt.Errorf("failed to scan multilock.expires: %w", err)
			}
			if expires.Before(thePast) {
				return ErrAlreadyLocked
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	logger.Debugw("acquired locks", "locks", lockOrder)
	return makeMultiUnlockFn(ctx, db, lockOrder, expires), nil
}

func makeMultiUnlockFn(ctx context.Context, db *DB, lockIDs []string, expires time.Time) UnlockFn {
	logger := logging.FromContext(ctx).Named("database.MultiUnlock")

	return func() error {
		return db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
			for _, lockID := range lockIDs {
				logger := logger.With("lock_id", lockID)

				row := tx.QueryRow(ctx, `SELECT ReleaseLock($1, $2)`, lockID, expires)
				var released bool
				if err := row.Scan(&released); err != nil {
					return fmt.Errorf("failed to scan lock.released: %w", err)
				}

				if released {
					logger.Debugw("released lock")
				} else {
					logger.Warnw("failed to release lock - it may have expired or no longer belongs to you")
				}
			}
			return nil
		})
	}
}

// Lock acquires lock with given name that times out after ttl. Returns an
// UnlockFn that can be used to unlock the lock. ErrAlreadyLocked will be
// returned if there is already a lock in use.
func (db *DB) Lock(ctx context.Context, lockID string, ttl time.Duration) (UnlockFn, error) {
	logger := logging.FromContext(ctx).Named("database.Lock").
		With("lock_id", lockID)

	var expires time.Time
	if err := db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `SELECT AcquireLock($1, $2)`, lockID, int32(ttl.Seconds()))

		if err := row.Scan(&expires); err != nil {
			return fmt.Errorf("failed to scan lock.expires: %w", err)
		}

		if expires.Before(thePast) {
			return ErrAlreadyLocked
		}
		return nil
	}); err != nil {
		return nil, err
	}

	logger.Debugw("acquired lock")
	return makeUnlockFn(ctx, db, lockID, expires), nil
}

func makeUnlockFn(ctx context.Context, db *DB, lockID string, expires time.Time) UnlockFn {
	logger := logging.FromContext(ctx).Named("database.Unlock").
		With("lock_id", lockID)

	return func() error {
		return db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
			row := tx.QueryRow(ctx, `SELECT ReleaseLock($1, $2)`, lockID, expires)
			var released bool
			if err := row.Scan(&released); err != nil {
				return fmt.Errorf("failed to scan lock.released: %w", err)
			}

			if released {
				logger.Debugw("released lock")
			} else {
				logger.Warnw("failed to release lock - it may have expired or no longer belongs to you")
			}
			return nil
		})
	}
}
