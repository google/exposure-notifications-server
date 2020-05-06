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

package database

import (
	"context"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/model"
	"github.com/google/go-cmp/cmp"
)

func TestAddExportConfig(t *testing.T) {
	if testDB == nil {
		t.Skip("no test DB")
	}
	ctx := context.Background()
	fromTime := time.Now().UTC()
	thruTime := fromTime.Add(6 * time.Hour)
	want := &model.ExportConfig{
		FilenameRoot:   "root",
		Period:         3 * time.Hour,
		IncludeRegions: []string{"i1", "i2"},
		ExcludeRegions: nil,
		From:           fromTime,
		Thru:           thruTime,
	}
	if err := testDB.AddExportConfig(ctx, want); err != nil {
		t.Fatal(err)
	}
	conn, err := testDB.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	var (
		got   model.ExportConfig
		psecs int
	)
	err = conn.QueryRow(ctx, `
		SELECT
			config_id, filename_root, period_seconds, include_regions, exclude_regions, from_timestamp, thru_timestamp
		FROM
			ExportConfig
		WHERE
			config_id = $1
	`, want.ConfigID).Scan(&got.ConfigID, &got.FilenameRoot, &psecs, &got.IncludeRegions, &got.ExcludeRegions, &got.From, &got.Thru)
	if err != nil {
		t.Fatal(err)
	}
	got.Period = time.Duration(psecs) * time.Second

	want.From = want.From.Truncate(time.Microsecond)
	want.Thru = want.Thru.Truncate(time.Microsecond)
	if diff := cmp.Diff(want, &got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}
