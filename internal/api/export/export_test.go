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

package export

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

type simpleBatchRange struct {
	start string
	end   string
}

// TestMakeBatchRanges tests makeBatchRanges().
func TestMakeBatchRanges(t *testing.T) {
	now := "12-10 10:11"

	testCases := []struct {
		name      string
		period    time.Duration
		latestEnd string
		want      []simpleBatchRange
	}{
		{
			name:      "no room for new batch",
			period:    1 * time.Hour,
			latestEnd: "12-10 10:00",
		},
		{
			name:      "no batches ever before so want just one batch",
			period:    1 * time.Hour,
			latestEnd: "",
			want:      []simpleBatchRange{{"12-10 09:00", "12-10 10:00"}},
		},
		{
			name:      "room for only one batch",
			period:    1 * time.Hour,
			latestEnd: "12-10 09:00",
			want:      []simpleBatchRange{{"12-10 09:00", "12-10 10:00"}},
		},
		{
			name:      "room for many batches",
			period:    1 * time.Hour,
			latestEnd: "12-10 07:00",
			want:      []simpleBatchRange{{"12-10 07:00", "12-10 08:00"}, {"12-10 08:00", "12-10 09:00"}, {"12-10 09:00", "12-10 10:00"}},
		},
		{
			name:      "room for many small batches",
			period:    15 * time.Minute,
			latestEnd: "12-10 09:00",
			want:      []simpleBatchRange{{"12-10 09:00", "12-10 09:15"}, {"12-10 09:15", "12-10 09:30"}, {"12-10 09:30", "12-10 09:45"}, {"12-10 09:45", "12-10 10:00"}},
		},
		{
			name:      "daily batch",
			period:    24 * time.Hour,
			latestEnd: "12-09 00:00",
			want:      []simpleBatchRange{{"12-09 00:00", "12-10 00:00"}},
		},
		{
			name:      "daily batch with room for many",
			period:    24 * time.Hour,
			latestEnd: "12-08 00:00",
			want:      []simpleBatchRange{{"12-08 00:00", "12-09 00:00"}, {"12-09 00:00", "12-10 00:00"}},
		},
		{
			name:      "daily batch with no previous batches",
			period:    24 * time.Hour,
			latestEnd: "",
			want:      []simpleBatchRange{{"12-09 00:00", "12-10 00:00"}},
		},
		{
			name:      "new batch overlaps with previous if misaligned",
			period:    1 * time.Hour,
			latestEnd: "12-10 09:15",
			want:      []simpleBatchRange{{"12-10 09:00", "12-10 10:00"}},
		},
		{
			name:      "small export window doesn't overlap open publish window",
			period:    time.Minute,
			latestEnd: "12-10 9:58",
			want:      []simpleBatchRange{{"12-10 09:58", "12-10 09:59"}, {"12-10 09:59", "12-10 10:00"}},
		},
		{
			name:      "small export window with no history does not create a batch overlapping open publish window",
			period:    10 * time.Minute,
			latestEnd: "",
			want:      []simpleBatchRange{{"12-10 09:50", "12-10 10:00"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nowT := fromSimpleTime(t, now)
			latestEndT := fromSimpleTime(t, tc.latestEnd)

			got := makeBatchRanges(tc.period, latestEndT, nowT)

			if len(got) != len(tc.want) {
				t.Errorf("incorrect number of batches got %v, want %v", toSimpleBatchRange(t, got), tc.want)
				return
			}

			for i := range got {
				wantStartT := fromSimpleTime(t, tc.want[i].start)
				wantEndT := fromSimpleTime(t, tc.want[i].end)

				if !got[i].start.Equal(wantStartT) {
					t.Errorf("unexpected start time for index %d, got %v, want %v", i, toSimpleTime(t, got[i].start), toSimpleTime(t, wantStartT))
				}
				if !got[i].end.Equal(wantEndT) {
					t.Errorf("unexpected end time for index %d, got %v, want %v", i, toSimpleTime(t, got[i].end), toSimpleTime(t, wantEndT))
				}
				if got[i].end.Sub(got[i].start) != tc.period {
					t.Errorf("unexpected range difference between start and end for index %d, got %v, want %v", i, got[i].end.Sub(got[i].start), tc.period)
				}
			}

		})
	}
}

func fromSimpleTime(t *testing.T, s string) time.Time {
	t.Helper()
	if s == "" {
		return time.Time{}
	}
	// Assume string is of the format "MM-DD HH:mm"; convert to RFC3339.
	rfc := "2020-" + strings.Replace(s, " ", "T", 1) + ":00Z"
	tm, err := time.Parse(time.RFC3339, rfc)
	if err != nil {
		t.Fatalf("Failed to parse simple time %q which was converted to %q: %v", s, rfc, err)
	}
	return tm
}

func toSimpleTime(t *testing.T, tm time.Time) string {
	t.Helper()
	return fmt.Sprintf("%02d-%02d %02d:%02d", tm.Month(), tm.Day(), tm.Hour(), tm.Minute())
}

func toSimpleBatchRange(t *testing.T, batches []batchRange) []simpleBatchRange {
	t.Helper()
	var simple []simpleBatchRange
	for _, br := range batches {
		simple = append(simple, simpleBatchRange{start: toSimpleTime(t, br.start), end: toSimpleTime(t, br.end)})
	}
	return simple
}
