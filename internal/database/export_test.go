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
	"sort"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/model"
	"github.com/google/go-cmp/cmp"
)

func TestAddExportConfig(t *testing.T) {
	if testDB == nil {
		t.Skip("no test DB")
	}
	defer resetTestDB(t)
	ctx := context.Background()

	fromTime := time.Now().UTC()
	thruTime := fromTime.Add(6 * time.Hour)
	want := &model.ExportConfig{
		FilenameRoot: "root",
		Period:       3 * time.Hour,
		Region:       "i1",
		From:         fromTime,
		Thru:         thruTime,
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
			config_id, filename_root, period_seconds, region, from_timestamp, thru_timestamp
		FROM
			ExportConfig
		WHERE
			config_id = $1
	`, want.ConfigID).Scan(&got.ConfigID, &got.FilenameRoot, &psecs, &got.Region, &got.From, &got.Thru)
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

func TestIterateExportConfigs(t *testing.T) {
	if testDB == nil {
		t.Skip("no test DB")
	}
	defer resetTestDB(t)
	ctx := context.Background()

	now := time.Now().Truncate(time.Microsecond)
	ecs := []*model.ExportConfig{
		{
			FilenameRoot: "active 1",
			From:         now.Add(-time.Minute),
			Thru:         now.Add(time.Minute),
		},
		{
			FilenameRoot: "active 2",
			From:         now.Add(-time.Minute),
		},
		{
			FilenameRoot: "done",
			From:         now.Add(-time.Hour),
			Thru:         now.Add(-time.Minute),
		},
		{
			FilenameRoot: "not yet",
			From:         now.Add(time.Minute),
			Thru:         now.Add(time.Hour),
		},
	}
	for _, ec := range ecs {
		ec.Period = time.Hour
		ec.Region = "R"
		if err := testDB.AddExportConfig(ctx, ec); err != nil {
			t.Fatal(err)
		}
	}

	iter, err := testDB.IterateExportConfigs(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := iter.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	var got []*model.ExportConfig
	for {
		ec, done, err := iter.Next()
		if err != nil {
			t.Fatal(err)
		}
		if done {
			break
		}
		got = append(got, ec)
	}
	want := ecs[0:2]
	sort.Slice(got, func(i, j int) bool { return got[i].FilenameRoot < got[j].FilenameRoot })
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}
