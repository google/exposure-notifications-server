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

// Package timeutils defines functions to close the gaps present in Golang's
// default implementation of Time.
package timeutils

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestLocalMidnight(t *testing.T) {
	t.Parallel()

	day := time.Date(2020, 10, 31, 4, 15, 0, 0, time.Local)
	want := time.Date(2020, 10, 31, 0, 0, 0, 0, time.Local)
	got := LocalMidnight(day)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}

	got = Midnight(day)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestUTCMidnight(t *testing.T) {
	t.Parallel()

	day := time.Date(2020, 10, 31, 4, 15, 0, 0, time.Local)
	want := time.Date(2020, 10, 31, 0, 0, 0, 0, time.UTC)
	got := UTCMidnight(day)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}
