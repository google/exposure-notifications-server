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

package v1

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestTotal(t *testing.T) {
	p := &PublishRequests{
		UnknownPlatform: 6,
		Android:         22,
		IOS:             45,
	}
	if want, got := int64(73), p.Total(); want != got {
		t.Fatalf("addition not working, want: %v got: %v", want, got)
	}
}

func TestStatsDays_MarshalCSV(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		stats StatsDays
		exp   string
	}{
		{
			name:  "empty",
			stats: nil,
			exp:   "",
		},
		{
			name: "single",
			stats: []*StatsDay{
				{
					Day: time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
					PublishRequests: PublishRequests{
						UnknownPlatform: 1,
						Android:         2,
						IOS:             3,
					},
					TotalTEKsPublished:        10,
					RevisionRequests:          9,
					TEKAgeDistribution:        []int64{2, 4, 5},
					OnsetToUploadDistribution: []int64{1, 3, 4},
					RequestsMissingOnsetDate:  7,
				},
			},
			exp: `day,publish_requests_unknown,publish_requests_android,publish_requests_ios,total_teks_published,requests_with_revisions,requests_missing_onset_date,tek_age_distribution,onset_to_upload_distribution
2020-02-03,1,2,3,10,9,7,2|4|5,1|3|4
`,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b, err := tc.stats.MarshalCSV()
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(string(b), tc.exp); diff != "" {
				t.Errorf("bad csv (+got, -want): %s", diff)
			}
		})
	}
}
