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

// Package timeutils defines functions to close the gaps present in Golang's
// default implementation of Time.
package timeutils

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestSubtractDays(t *testing.T) {
	t.Parallel()

	day := time.Date(2020, 10, 31, 4, 15, 0, 0, time.UTC)

	cases := []struct {
		name string
		days uint
		want time.Time
	}{
		{
			name: "zero",
			days: 0,
			want: day,
		},
		{
			name: "fortnight",
			days: 14,
			want: time.Date(2020, 10, 17, 4, 15, 0, 0, time.UTC),
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := SubtractDays(day, tc.days)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
