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
	"time"

	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/go-cmp/cmp"
)

func TestNewFromEnv(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	t.Run("bad_conn", func(t *testing.T) {
		t.Parallel()

		if _, err := NewFromEnv(ctx, &Config{}); err == nil {
			t.Errorf("expected error, got nil")
		}
	})
}

func TestDBValues(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		config Config
		want   map[string]string
	}{
		{
			name: "empty configs",
			want: make(map[string]string),
		},
		{
			name: "some config",
			config: Config{
				Name:              "myDatabase",
				User:              "superuser",
				Password:          "notAG00DP@ssword",
				Port:              "1234",
				ConnectionTimeout: 5,
				PoolHealthCheck:   5 * time.Minute,
			},
			want: map[string]string{
				"dbname":                   "myDatabase",
				"password":                 "notAG00DP@ssword",
				"port":                     "1234",
				"user":                     "superuser",
				"connect_timeout":          "5",
				"pool_health_check_period": "5m0s",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := dbValues(&tc.config)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
