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
)

func TestCutoffDate(t *testing.T) {
	now := time.Now()
	for _, test := range []struct {
		d       time.Duration
		wantDur time.Duration // if zero, then expect an error
	}{
		{216 * time.Hour, 0},                       // 9 days: duration too short
		{-10 * time.Minute, 0},                     // negative
		{241 * time.Hour, (10*24 + 1) * time.Hour}, // 10 days, 1 hour: OK
	} {
		got, err := cutoffDate(test.d)
		if test.wantDur == 0 {
			if err == nil {
				t.Errorf("%q: got no error, wanted one", test.d)
			}
		} else if err != nil {
			t.Errorf("%q: got error %v", test.d, err)
		} else {
			want := now.Add(-test.wantDur)
			diff := got.Sub(want)
			if diff < 0 {
				diff = -diff
			}
			if diff > time.Second {
				t.Errorf("%q: got %s, want %s", test.d, got, want)
			}
		}
	}
}
