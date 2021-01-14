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

package flag

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value []string
		want  string
	}{
		{
			name:  "empty",
			value: []string{},
			want:  "[]",
		},
		{
			name:  "single",
			value: []string{"A"},
			want:  "[A]",
		},
		{
			name:  "multi",
			value: []string{"A", "B", "C"},
			want:  "[A B C]",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var l RegionListVar = tc.value
			if r := l.String(); r != tc.want {
				t.Errorf("wrong value, want: %q got: %q", tc.want, r)
			}
		})
	}
}

func TestSetError(t *testing.T) {
	t.Parallel()

	var l RegionListVar = []string{"A"}
	if err := l.Set("A"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSet(t *testing.T) {
	t.Parallel()

	var l RegionListVar
	if err := l.Set("A, B, C,D, A"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var want RegionListVar = []string{"A", "B", "C", "D"}
	if diff := cmp.Diff(want, l); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}
