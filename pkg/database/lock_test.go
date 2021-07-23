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
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/project"
)

func TestLock(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)

	const (
		id1 = "test1"
		id2 = "test2"
	)

	mustLock := func(id string, ttl time.Duration) UnlockFn {
		t.Helper()
		unlock, err := testDB.Lock(ctx, id, ttl)
		if err != nil {
			t.Fatal(err)
		}
		return unlock
	}

	//  Grab a free lock.
	unlock1 := mustLock(id1, time.Hour)

	// Fail to grab a held lock.
	if _, err := testDB.Lock(ctx, id1, time.Hour); !errors.Is(err, ErrAlreadyLocked) {
		t.Fatalf("got %v, wanted ErrAlreadyLocked", err)
	}
	unlock2 := mustLock(id2, time.Hour)
	// Unlock the first lock.
	if err := unlock1(); err != nil {
		t.Fatal(err)
	}

	// Re-acquire the first lock, briefly.
	_ = mustLock(id1, time.Microsecond)

	// We can acquire the lock after it expires.
	time.Sleep(50 * time.Millisecond)
	unlock1 = mustLock(id1, time.Hour)

	// Unlock both locks.
	if err := unlock1(); err != nil {
		t.Fatal(err)
	}
	if err := unlock2(); err != nil {
		t.Fatal(err)
	}

	// Lock table should be empty.
	conn, err := testDB.Pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()

	var expires time.Time
	if err := conn.QueryRow(ctx, `SELECT expires FROM Lock WHERE lock_id = $1`, id1).Scan(&expires); err != nil {
		t.Fatal(err)
	}
	if got, want := expires.UTC(), time.Unix(0, 0).UTC(); got != want {
		t.Fatalf("expected lock to be expired (%v), got %v", want, got)
	}
}

// TestLock_contention attempts to test that high lock contention does not
// result in database-level errors, specifically with respect to transaction
// isolation levels and dirty reads.
func TestLock_contention(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)

	var wg sync.WaitGroup
	errCh := make(chan error, 100)
	for i := 0; i < 10; i++ {
		lockID := fmt.Sprintf("lock_%d", i)

		for j := 0; j < 5; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				unlock, err := testDB.Lock(ctx, lockID, 5*time.Second)
				if err != nil {
					select {
					case errCh <- fmt.Errorf("failed to lock %s: %w", lockID, err):
					default:
					}
				}
				if unlock != nil {
					if err := unlock(); err != nil {
						select {
						case errCh <- fmt.Errorf("failed to unlock %s: %w", lockID, err):
						default:
						}
					}
				}
			}()
		}
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil && !errors.Is(err, ErrAlreadyLocked) {
			t.Error(err)
		}
	}
}

func TestMultiLock(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)

	if _, err := testDB.MultiLock(ctx, nil, time.Minute); err == nil {
		t.Errorf("expected error, got nil")
	}

	neededLocks := []string{"traveler", "US", "CA", "MX"}

	unlock1, err := testDB.MultiLock(ctx, neededLocks, time.Minute)
	if err != nil {
		t.Fatalf("failed to obtain locks, %v, err: %v", neededLocks, err)
	}

	overlappingLocks := []string{"CA", "CH", "UK"}

	if _, err := testDB.MultiLock(ctx, overlappingLocks, time.Minute); err == nil {
		t.Fatalf("expected lock acquisition to fail, but didn't.")
	} else if !errors.Is(err, ErrAlreadyLocked) {
		t.Fatalf("wong error want: %v, got: %v", ErrAlreadyLocked, err)
	}

	nonoverlappingLocks := []string{"CH", "UK"}
	unlock2, err := testDB.MultiLock(ctx, nonoverlappingLocks, time.Minute)
	if err != nil {
		t.Fatalf("failed to obtain locks, %v, err: %v", nonoverlappingLocks, err)
	}

	if err := unlock1(); err != nil {
		t.Fatalf("failed to release locks: %v", err)
	}

	// should still fail, because there is still overlap w/ the second lock.
	if _, err := testDB.MultiLock(ctx, overlappingLocks, time.Minute); err == nil {
		t.Fatalf("expected lock acquisition to fail, but didn't.")
	} else if !errors.Is(err, ErrAlreadyLocked) {
		t.Fatalf("wong error want: %v, got: %v", ErrAlreadyLocked, err)
	}

	if err := unlock2(); err != nil {
		t.Fatalf("failed to release locks: %v", err)
	}
}
