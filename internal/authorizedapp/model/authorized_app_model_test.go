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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestAuthorizedApp_IsAllowedRegion(t *testing.T) {
	t.Parallel()

	cfg := NewAuthorizedApp()
	cfg.AllowedRegions = map[string]struct{}{
		"US": {},
	}

	if ok := cfg.IsAllowedRegion("US"); !ok {
		t.Errorf("expected US to be allowed")
	}

	if ok := cfg.IsAllowedRegion("CA"); ok {
		t.Errorf("expected CA to not be allowed")
	}
}

func TestAllAllowedRegions(t *testing.T) {
	t.Parallel()

	cfg := NewAuthorizedApp()
	cfg.AllowedRegions = map[string]struct{}{
		"US": {},
		"CA": {},
	}

	got := cfg.AllAllowedRegions()
	want := []string{"US", "CA"}
	sorter := cmpopts.SortSlices(func(a, b string) bool {
		return strings.Compare(a, b) <= 0
	})
	if diff := cmp.Diff(want, got, sorter); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestRegionsOnePerLine(t *testing.T) {
	t.Parallel()

	cfg := NewAuthorizedApp()
	cfg.AllowedRegions = map[string]struct{}{
		"US": {},
		"CA": {},
	}

	got := cfg.RegionsOnePerLine()
	want := "CA\nUS"
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestAllAllowedHealthAuthorityIDs(t *testing.T) {
	t.Parallel()

	cfg := NewAuthorizedApp()
	cfg.AllowedHealthAuthorityIDs[12] = struct{}{}
	cfg.AllowedHealthAuthorityIDs[42] = struct{}{}

	got := cfg.AllAllowedHealthAuthorityIDs()
	want := []int64{12, 42}
	sorter := cmpopts.SortSlices(func(a, b int64) bool {
		return a <= b
	})
	if diff := cmp.Diff(want, got, sorter); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	cfg := NewAuthorizedApp()
	got := cfg.Validate()
	want := []string{
		"Health Authority ID cannot be empty",
		"Regions list cannot be empty",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestIsAllowedRegions(t *testing.T) {
	t.Parallel()

	cfg := NewAuthorizedApp()

	if !cfg.IsAllowedRegion("foo") {
		t.Errorf("region disallowed when all regions should be allowed")
	}

	cfg.AllowedRegions = map[string]struct{}{
		"US": {},
		"CA": {},
	}

	if !cfg.IsAllowedRegion("US") {
		t.Errorf("missing expected allowed region: US")
	}
	if !cfg.IsAllowedRegion("CA") {
		t.Errorf("missing expected allowed region: CA")
	}
	if cfg.IsAllowedRegion("GB") {
		t.Errorf("unexpected allowed region: GB")
	}
}
