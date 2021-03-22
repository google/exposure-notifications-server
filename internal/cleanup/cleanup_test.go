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

package cleanup

import (
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/database"
)

var testDatabaseInstance *database.TestInstance

func TestMain(m *testing.M) {
	testDatabaseInstance = database.MustTestInstance()
	defer testDatabaseInstance.MustClose()
	m.Run()
}

func TestCutoffDate(t *testing.T) {
	t.Parallel()

	now := time.Now()

	cases := []struct {
		name     string
		d        time.Duration
		wantDur  time.Duration // if zero, then expect an error
		override bool
	}{
		{"too_short", 216 * time.Hour, 0, false},                                 // 9 days: duration too short
		{"negative", -10 * time.Minute, 0, false},                                // negative
		{"long_enough", 241 * time.Hour, (10*24 + 1) * time.Hour, false},         // 10 days, 1 hour: OK
		{"too_short_with_override", 216 * time.Hour, (9 * 24) * time.Hour, true}, // too short, but override backstop.
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := cutoffDate(tc.d, tc.override)
			if tc.wantDur == 0 {
				if err == nil {
					t.Errorf("%q: got no error, wanted one", tc.d)
				}
			} else if err != nil {
				t.Errorf("%q: got error %v", tc.d, err)
			} else {
				want := now.Add(-tc.wantDur)
				diff := got.Sub(want)
				if diff < 0 {
					diff = -diff
				}
				if diff > time.Second {
					t.Errorf("%q: got %s, want %s", tc.d, got, want)
				}
			}
		})
	}
}
