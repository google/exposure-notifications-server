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

// Package cache implements an inmemory cache for any interface{} object.
package cache

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

type order struct {
	Burgers int
	Fries   int
}

func checkSize(t *testing.T, c *Cache, want int) {
	t.Helper()

	if got := c.Size(); got != want {
		t.Errorf("wrong size want: %v, got: %v", want, got)
	}
}

func TestCache(t *testing.T) {
	cache := New()

	checkSize(t, cache, 0)

	if err := cache.Set("foo", &order{2, 1}, time.Millisecond); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	time.Sleep(time.Millisecond)
	if got, hit := cache.Lookup("foo"); got != nil || hit {
		t.Fatalf("key did not expire as expected")
	}

	if got, hit := cache.Lookup("bar"); got != nil || hit {
		t.Fatalf("got key that was never inserted")
	}

	want := &order{42, 37}
	if err := cache.Set("foo", want, time.Second*2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, hit := cache.Lookup("foo"); got == nil || !hit {
		t.Fatalf("lookup failed want: %v, got %v", want, got)
	} else {
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("mismatch (-want, +got):\n%s", diff)
		}
	}
	time.Sleep(time.Second * 2)
	if got, hit := cache.Lookup("foo"); got != nil || hit {
		t.Fatalf("expected key to expire, but still available")
	}
	// potential race, yeild CPU so that the purge go routine has a chance to run.
	time.Sleep(time.Millisecond * 500)
	checkSize(t, cache, 0)
}

func TestWriteThruCache(t *testing.T) {
	cache := New()

	lookupCount := 0
	want := &order{12, 34}
	lookerUpper := func() (interface{}, error) {
		lookupCount++
		return want, nil
	}

	for i := 0; i < 2; i++ {
		got, err := cache.WriteThruLookup("foo", lookerUpper, time.Second)
		if err != nil {
			t.Fatalf("unexpected error on WriteThruLookup: %v", err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("mismatch (-want, +got):\n%s", diff)
		}
	}

	if lookupCount != 1 {
		t.Fatalf("incorrect lookup count, want: 1, got: %v", lookupCount)
	}
}

func TestWriteThruError(t *testing.T) {
	cache := New()

	lookerUpper := func() (interface{}, error) {
		return nil, fmt.Errorf("nope")
	}

	got, err := cache.WriteThruLookup("foo", lookerUpper, time.Second)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "nope" {
		t.Errorf("incorrect error, want: `nope` got: %v", err.Error())
	}
	if got != nil {
		t.Errorf("unexpecetd cached item, want: nil, got %v", got)
	}
}

func TestInvalidDuration(t *testing.T) {
	cache := New()

	if err := cache.Set("foo", &order{0, 0}, -1*time.Second); err == nil {
		t.Fatal("expecterd error, got nil")
	} else if strings.Contains(err.Error(), "duration cannot be nagative") {
		t.Fatalf("wrong error: want: `duration cannot be negative` got: %v", err.Error())
	}
}
