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

func TextExportRegions(t *testing.T) {
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

func TestExportConfigFormatting(t *testing.T) {
	cases := []struct {
		name              string
		ec                *ExportConfig
		formattedFromTime string
		formattedThruTime string
		fromHTMLDate      string
		fromHTMLTime      string
		thruHTMLDate      string
		thruHTMLTime      string
	}{
		{
			name: "both empty",
			ec: &ExportConfig{
				From: time.Time{},
				Thru: time.Time{},
			},
			formattedFromTime: "Mon Jan  1 00:00:00 UTC 0001",
		},
		{
			name: "not data",
			ec: &ExportConfig{
				From: time.Date(2020, 05, 01, 12, 01, 02, 0, time.UTC),
				Thru: time.Date(2020, 06, 01, 12, 01, 02, 0, time.UTC),
			},
			formattedFromTime: "Fri May  1 12:01:02 UTC 2020",
			formattedThruTime: "Mon Jun  1 12:01:02 UTC 2020",
			fromHTMLDate:      "2020-05-01",
			fromHTMLTime:      "12:01",
			thruHTMLDate:      "2020-06-01",
			thruHTMLTime:      "12:01",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.ec.FormattedFromTime(); tc.formattedFromTime != got {
				t.Errorf("FormattedFromTime want: '%v' got '%v'", tc.formattedFromTime, got)
			}
			if got := tc.ec.FormattedThruTime(); tc.formattedThruTime != got {
				t.Errorf("FormattedThruTime want: '%v' got '%v'", tc.formattedThruTime, got)
			}
			if got := tc.ec.FromHTMLDate(); tc.fromHTMLDate != got {
				t.Errorf("FromHTMLDate want: '%v' got '%v'", tc.fromHTMLDate, got)
			}
			if got := tc.ec.FromHTMLTime(); tc.fromHTMLTime != got {
				t.Errorf("FromHTMLTime want: '%v' got '%v'", tc.fromHTMLTime, got)
			}
			if got := tc.ec.ThruHTMLDate(); tc.thruHTMLDate != got {
				t.Errorf("ThruHTMLDate want: '%v' got '%v'", tc.thruHTMLDate, got)
			}
			if got := tc.ec.ThruHTMLTime(); tc.thruHTMLTime != got {
				t.Errorf("ThruHTMLTime want: '%v' got '%v'", tc.thruHTMLTime, got)
			}
		})
	}
}
