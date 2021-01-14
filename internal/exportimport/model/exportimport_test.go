// Copyright 2021 Google LLC
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

func timePtr(t time.Time) *time.Time {
	return &t
}

func TestValidate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ei   *ExportImport
		want string
	}{
		{
			name: "no_region",
			ei:   &ExportImport{},
			want: "region cannot be blank",
		},
		{
			name: "too_big_region",
			ei: &ExportImport{
				Region: "ABCDEF",
			},
			want: "region cannot be longer than 5 characters",
		},
		{
			name: "no_index_file",
			ei: &ExportImport{
				Region: "US",
			},
			want: "IndexFile cannot be blank",
		},
		{
			name: "no_export_root",
			ei: &ExportImport{
				Region:    "US",
				IndexFile: "a/index.txt",
			},
			want: "ExportRoot cannot be blank",
		},
		{
			name: "valid",
			ei: &ExportImport{
				Region:     "US",
				IndexFile:  "a/index.txt",
				ExportRoot: "a",
			},
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ""
			err := tc.ei.Validate()
			if err != nil {
				got = err.Error()
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("unmarshal mismatch (-want +got):\n%v", diff)
			}
		})
	}
}

func TestActive(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	cases := []struct {
		name string
		ei   *ExportImport
		want bool
	}{
		{
			name: "valid_time",
			ei: &ExportImport{
				From: now.Add(-1 * time.Second),
			},
			want: true,
		},
		{
			name: "valid_time_with_exp",
			ei: &ExportImport{
				From: now.Add(-1 * time.Second),
				Thru: timePtr(now.Add(time.Hour)),
			},
			want: true,
		},
		{
			name: "expired",
			ei: &ExportImport{
				From: now.Add(-2 * time.Hour),
				Thru: timePtr(now.Add(-1 * time.Hour)),
			},
			want: false,
		},
		{
			name: "future",
			ei: &ExportImport{
				From: now.Add(2 * time.Hour),
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.ei.Active()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("unmarshal mismatch (-want +got):\n%v", diff)
			}
		})
	}
}
