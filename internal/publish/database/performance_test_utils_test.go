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
	"testing"

	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/internal/publish/model"
)

func TestBulkInsertExposures(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		exposures []*model.Exposure
		want      int
	}{
		{
			name:      "No Exposures",
			exposures: []*model.Exposure{},
			want:      0,
		},
		{
			name: "Few Exposures",
			exposures: []*model.Exposure{
				{
					ExposureKey:    []byte("ABC"),
					Regions:        []string{"US", "CA", "MX"},
					IntervalNumber: 18,
				},
				{
					ExposureKey:    []byte("DEF"),
					Regions:        []string{"CA"},
					IntervalNumber: 118,
				},
				{
					ExposureKey:    []byte("123"),
					IntervalNumber: 218,
					Regions:        []string{"MX", "CA"},
				},
			},
			want: 3,
		},
	}

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	testPublishDB := New(testDB)

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			count, err := testPublishDB.BulkInsertExposures(ctx, tc.exposures)
			if err != nil {
				t.Fatal(err)
			}
			if count != tc.want {
				t.Errorf("Got %v Inserts, wanted %d", count, tc.want)
			}
		})
	}
}
