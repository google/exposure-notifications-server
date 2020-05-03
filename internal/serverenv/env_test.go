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

package serverenv

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestParseDurationEnv(t *testing.T) {
	ctx := context.Background()
	const varName = "PARSE_DURATION_TEST"
	const defaultValue = 17 * time.Second
	for _, test := range []struct {
		val  string
		want time.Duration
	}{
		{"", defaultValue},
		{"bad", defaultValue},
		{"250ms", 250 * time.Millisecond},
	} {
		os.Setenv(varName, test.val)
		got := ParseDuration(ctx, varName, defaultValue)
		if got != test.want {
			t.Errorf("%q: got %v, want %v", test.val, got, test.want)
		}
	}
}
