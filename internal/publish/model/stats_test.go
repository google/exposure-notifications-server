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

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/timeutils"
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

func minPadSlice(s []int64, size int) []int64 {
	if len(s) >= size {
		return s
	}
	resized := make([]int64, size)
	copy(resized, s)
	return resized
}

func TestReduce(t *testing.T) {

	hour := timeutils.UTCMidnight(time.Now().UTC()).Add(-48 * time.Hour)
	startTime := hour

	input := []*HealthAuthorityStats{
		// two entries from 2 days ago
		{
			HealthAuthorityID: 42,
			Hour:              hour,
			PublishCount:      []int64{1, 10, 4},
			TEKCount:          112,
			RevisionCount:     0,
			OldestTekDays:     []int64{0, 0, 0, 0, 0, 1, 0, 0, 1, 0, 0, 2, 0, 0, 10, 1},
			OnsetAgeDays:      minPadSlice([]int64{1, 1, 2, 5, 3, 1}, StatsMaxOnsetDays+1),
			MissingOnset:      1,
		},
		{
			HealthAuthorityID: 42,
			Hour:              hour.Add(time.Hour),
			PublishCount:      []int64{0, 5, 6},
			TEKCount:          65,
			RevisionCount:     0,
			OldestTekDays:     []int64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 11, 0},
			OnsetAgeDays:      minPadSlice([]int64{0, 0, 3, 5, 3}, StatsMaxOnsetDays+1),
			MissingOnset:      0,
		},
		// one entry from 1 days ago, but not enough uploads to be shown
		{
			HealthAuthorityID: 42,
			Hour:              hour.Add(24 * time.Hour),
			PublishCount:      []int64{1, 1, 1},
			TEKCount:          42,
			RevisionCount:     0,
			OldestTekDays:     []int64{0, 0, 0, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 0, 0},
			OnsetAgeDays:      minPadSlice([]int64{0, 0, 1, 1, 1}, StatsMaxOnsetDays+1),
			MissingOnset:      0,
		},
		// two entries from today, but one shouldn't be shown (time hasn't lapsed yet)
		{
			HealthAuthorityID: 42,
			Hour:              hour.Add(48 * time.Hour),
			PublishCount:      []int64{0, 187, 294},
			TEKCount:          6734,
			RevisionCount:     0,
			OldestTekDays:     []int64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 81, 200, 200, 0},
			OnsetAgeDays:      minPadSlice([]int64{1, 50, 100, 300, 29}, StatsMaxOnsetDays+1),
			MissingOnset:      0,
		},
		{ // This record should be held back.
			HealthAuthorityID: 42,
			Hour:              hour.Add(49 * time.Hour),
			PublishCount:      []int64{0, 5, 6},
			TEKCount:          65,
			RevisionCount:     0,
			OldestTekDays:     []int64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 11, 0},
			OnsetAgeDays:      minPadSlice([]int64{0, 0, 3, 5, 3}, StatsMaxOnsetDays+1),
			MissingOnset:      0,
		},
	}

	want := []*verifyapi.MetricsDay{
		{
			Day: startTime,
			PublishCount: verifyapi.PublishCount{
				UnknownPlatform: 1,
				Android:         15,
				IOS:             10,
			},
			TEKs:                  177,
			Revisions:             0,
			OldestTEKDistribution: []int64{0, 0, 0, 0, 0, 1, 0, 0, 1, 0, 0, 2, 0, 0, 21, 1},
			OnsetToUpload:         minPadSlice([]int64{1, 1, 5, 10, 6, 1}, StatsMaxOnsetDays+1),
			MissingOnset:          1,
		},
		{
			Day: startTime.Add(48 * time.Hour),
			PublishCount: verifyapi.PublishCount{
				UnknownPlatform: 0,
				Android:         187,
				IOS:             294,
			},
			TEKs:                  6734,
			Revisions:             0,
			OldestTEKDistribution: []int64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 81, 200, 200, 0},
			OnsetToUpload:         minPadSlice([]int64{1, 50, 100, 300, 29}, StatsMaxOnsetDays+1),
			MissingOnset:          0,
		},
	}

	got := ReduceStats(input, hour.Add(49*time.Hour), 10)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}
