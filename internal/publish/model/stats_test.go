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

package model

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestCheckAddPublish(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Hour)
	record := InitHour(1, now)

	want := InitHour(1, now)
	compare(want, record, t)

	{
		info := PublishInfo{
			Platform:     PlatformAndroid,
			NumTEKs:      14,
			Revision:     false,
			OldestDays:   14,
			OnsetDaysAgo: 4,
			MissingOnset: false,
		}

		record.AddPublish(&info)

		want = &HealthAuthorityStats{
			HealthAuthorityID: want.HealthAuthorityID,
			Hour:              want.Hour,
			PublishCount:      []int64{0, 1, 0},
			TEKCount:          14,
			RevisionCount:     0,
			OldestTekDays:     []int64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0},
			OnsetAgeDays:      []int64{0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			MissingOnset:      0,
		}
		compare(want, record, t)
	}

	{
		info := PublishInfo{
			Platform:     PlatformIOS,
			NumTEKs:      10,
			Revision:     true,
			OldestDays:   10,
			OnsetDaysAgo: 3,
			MissingOnset: false,
		}

		record.AddPublish(&info)

		want = &HealthAuthorityStats{
			HealthAuthorityID: want.HealthAuthorityID,
			Hour:              want.Hour,
			PublishCount:      []int64{0, 1, 1},
			TEKCount:          24,
			RevisionCount:     1,
			OldestTekDays:     []int64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0},
			OnsetAgeDays:      []int64{0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			MissingOnset:      0,
		}
		compare(want, record, t)
	}

	{
		info := PublishInfo{
			Platform:     "palm",
			NumTEKs:      5,
			Revision:     false,
			OldestDays:   5,
			OnsetDaysAgo: 4,
			MissingOnset: true,
		}

		record.AddPublish(&info)

		want = &HealthAuthorityStats{
			HealthAuthorityID: want.HealthAuthorityID,
			Hour:              want.Hour,
			PublishCount:      []int64{1, 1, 1},
			TEKCount:          29,
			RevisionCount:     1,
			OldestTekDays:     []int64{0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0},
			OnsetAgeDays:      []int64{0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			MissingOnset:      1,
		}
		compare(want, record, t)
	}

	{
		info := PublishInfo{
			Platform:     PlatformAndroid,
			NumTEKs:      20,
			Revision:     false,
			OldestDays:   StatsMaxOldestTEK + 5,
			OnsetDaysAgo: StatsMaxOnsetDays + 5,
			MissingOnset: false,
		}

		record.AddPublish(&info)

		want = &HealthAuthorityStats{
			HealthAuthorityID: want.HealthAuthorityID,
			Hour:              want.Hour,
			PublishCount:      []int64{1, 2, 1},
			TEKCount:          49,
			RevisionCount:     1,
			OldestTekDays:     []int64{0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1},
			OnsetAgeDays:      []int64{0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			MissingOnset:      1,
		}
		compare(want, record, t)
	}
}

func compare(want, got *HealthAuthorityStats, t *testing.T) {
	t.Helper()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}
