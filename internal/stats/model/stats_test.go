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

package stats

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestHealthAuthorityStatsRelease(t *testing.T) {
	t.Parallel()

	h := &HealthAuthorityRelease{
		NotBefore: time.Time{},
	}

	if got := h.CanPull(); !got {
		t.Fatalf("can't pull never run stats release: %v", got)
	}

	h.NotBefore = time.Now().Add(time.Hour)

	if got := h.CanPull(); got {
		t.Fatalf("can pull future stats release: %v", got)
	}
}

func TestCheckAddPublish(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Hour)
	record := InitHour(1, now)

	want := InitHour(1, now)
	compare(want, record, t)

	record.AddPublish(14, false, 14, 4, false)

	want = &HealthAuthorityStats{
		HealthAuthorityID: want.HealthAuthorityID,
		Hour:              want.Hour,
		PublishCount:      1,
		TEKCount:          14,
		RevisionCount:     0,
		OldestTekDays:     []int32{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		OnsetAgeDays:      []int32{0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		MissingOnset:      0,
	}
	compare(want, record, t)
}

func compare(want, got *HealthAuthorityStats, t *testing.T) {
	t.Helper()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}
