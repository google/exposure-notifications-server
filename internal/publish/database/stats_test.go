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

	"github.com/google/exposure-notifications-server/internal/publish/model"
	hadb "github.com/google/exposure-notifications-server/internal/verification/database"
	hamodel "github.com/google/exposure-notifications-server/internal/verification/model"
)

func TestUpdateStats(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	testPublishDB := New(testDB)

	testHADB := hadb.New(testDB)
	healthAuthority := hamodel.HealthAuthority{
		Issuer:   "a",
		Audience: "b",
		Name:     "c",
	}
	if err := testHADB.AddHealthAuthority(ctx, &healthAuthority); err != nil {
		t.Fatalf("unable to cerate health authority: %v", err)
	}

	hour := time.Now().UTC().Truncate(time.Hour)

	info := &model.PublishInfo{
		Platform:     model.PlatformAndroid,
		NumTEKs:      14,
		Revision:     false,
		OldestDays:   14,
		OnsetDaysAgo: 4,
		MissingOnset: false,
	}

	if err := testPublishDB.UpdateStats(ctx, hour, healthAuthority.ID, info); err != nil {
		t.Fatalf("updating stats: %v", err)
	}

	// Update again - test conflict codepath.
	if err := testPublishDB.UpdateStats(ctx, hour, healthAuthority.ID, info); err != nil {
		t.Fatalf("updating stats: %v", err)
	}
}

func TestReadStats(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	testPublishDB := New(testDB)

	testHADB := hadb.New(testDB)
	healthAuthority := hamodel.HealthAuthority{
		Issuer:   "a",
		Audience: "b",
		Name:     "c",
	}
	if err := testHADB.AddHealthAuthority(ctx, &healthAuthority); err != nil {
		t.Fatalf("unable to cerate health authority: %v", err)
	}

	info := &model.PublishInfo{
		Platform:     model.PlatformAndroid,
		NumTEKs:      14,
		Revision:     false,
		OldestDays:   14,
		OnsetDaysAgo: 4,
		MissingOnset: false,
	}

	startTime := time.Now().UTC().Add(-10 * time.Hour).Truncate(time.Hour)
	endTime := startTime.Add(10 * time.Hour)

	for hour := startTime; !hour.After(endTime); hour = hour.Add(time.Hour) {
		if err := testPublishDB.UpdateStats(ctx, hour, healthAuthority.ID, info); err != nil {
			t.Fatalf("updating stats: %v", err)
		}
	}

	stats, err := testPublishDB.ReadStats(ctx, healthAuthority.ID)
	if err != nil {
		t.Fatalf("unexpected error reading stats: %v", err)
	}

	if len(stats) != 11 {
		t.Fatalf("added 11 hours of stats, got: %v", len(stats))
	}
}
