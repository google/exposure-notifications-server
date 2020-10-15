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

	"github.com/google/go-cmp/cmp"
)

func TestExportRegions(t *testing.T) {
	cases := []struct {
		name         string
		region       string
		inputRegions []string
		effective    []string
	}{
		{
			name:         "single input",
			region:       "US",
			inputRegions: []string{},
			effective:    []string{"US"},
		},
		{
			name:         "multiple",
			region:       "US",
			inputRegions: []string{"WA", "OR"},
			effective:    []string{"WA", "OR"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ec := ExportConfig{
				OutputRegion: tc.region,
				InputRegions: tc.inputRegions,
			}
			got := ec.EffectiveInputRegions()
			if diff := cmp.Diff(tc.effective, got); diff != "" {
				t.Fatalf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
