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
	"cambio/pkg/model"
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/datastore"
)

var (
	// ErrAlreadyLocked is returned if the lock is already in use.
	ErrAlreadyLocked = errors.New("lock already in use")
)

// UnlockFn can be deferred to release a lock.
type UnlockFn func() error

// Lock acquires lock with given name that times out after ttl. Returns an UnlockFn that can be used to unlock the lock. ErrAlreadyLocked will be returned if there is already a lock in use.
func Lock(ctx context.Context, name string, ttl time.Duration) (UnlockFn, error) {
	client := Connection()
	if client == nil {
		return nil, fmt.Errorf("unable to obtain database client")
	}

	key := lockKey(name)
	now := time.Now()
	expiry := now.Add(ttl)

	_, err := client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {

		var lock model.Lock
		if errg := tx.Get(key, &lock); errg != nil {
			if errg == datastore.ErrNoSuchEntity {
				if errp := put(tx, key, expiry); errp != nil {
					return errp
				}
				return nil
			}
			return fmt.Errorf("getting lock %s: %v", key, errg)
		}

		// The lock exists, check to see if it's expired.
		if now.After(lock.Expires) {
			// Put a new lock with a new expiry.
			if errp := put(tx, key, expiry); errp != nil {
				return errp
			}
			return nil
		}
		return ErrAlreadyLocked

	})
	if err != nil {
		return nil, err
	}

	unlock := func() error {
		_, err := client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
			if err1 := tx.Delete(key); err1 != nil {
				return fmt.Errorf("deleting lock %s: %v", key, err1)
			}
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	}

	return unlock, nil
}

func put(tx *datastore.Transaction, key *datastore.Key, expiry time.Time) error {
	if _, err := tx.Put(key, &model.Lock{Expires: expiry}); err != nil {
		return fmt.Errorf("putting lock %s: %v", key, err)
	}
	return nil
}

func lockKey(name string) *datastore.Key {
	return datastore.NameKey(model.LockTable, name, nil)
}
