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
	"os"
	"testing"
	"time"
)

func TestGetCutoff(t *testing.T) {
	now := time.Now()
	for _, test := range []struct {
		val     string
		wantDur time.Duration // if zero, then expect an error
	}{
		{"", 0},                           // no env var
		{"foo", 0},                        // invalid duration
		{"216h", 0},                       // 9 days: duration too short
		{"241h", (10*24 + 1) * time.Hour}, // 10 days, 1 hour: OK
	} {
		os.Setenv(ttlEnvVar, test.val)
		got, err := getCutoff(ttlEnvVar)
		if test.wantDur == 0 {
			if err == nil {
				t.Errorf("%q: got no error, wanted one", test.val)
			}
		} else if err != nil {
			t.Errorf("%q: got error %v", test.val, err)
		} else {
			want := now.Add(-test.wantDur)
			diff := got.Sub(want)
			if diff < 0 {
				diff = -diff
			}
			if diff > time.Second {
				t.Errorf("%q: got %s, want %s", test.val, got, want)
			}
		}
	}
}
