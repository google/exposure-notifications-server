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
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestDBValues(t *testing.T) {
	testCases := []struct {
		name    string
		configs []config
		env     map[string]string
		want    map[string]string
		wantErr bool
	}{
		{
			name: "empty configs",
		},
		{
			name: "required missing",
			configs: []config{
				{env: "AAA", part: "aaa", def: "", req: true},
			},
			wantErr: true,
		},
		{
			name: "required present",
			configs: []config{
				{env: "AAA", part: "aaa", def: "", req: true},
			},
			env:  map[string]string{"AAA": "a_a"},
			want: map[string]string{"aaa": "a_a"},
		},
		{
			name: "optional not required",
			configs: []config{
				{env: "AAA", part: "aaa", def: ""},
			},
		},
		{
			name: "no default string",
			configs: []config{
				{env: "AAA", part: "aaa", def: ""},
			},
		},
		{
			name: "default string",
			configs: []config{
				{env: "AAA", part: "aaa", def: "default"},
			},
			want: map[string]string{"aaa": "default"},
		},
		{
			name: "valid enum",
			configs: []config{
				{env: "AAA", part: "aaa", def: "", valid: []string{"a1", "a2"}},
			},
			env:  map[string]string{"AAA": "a2"},
			want: map[string]string{"aaa": "a2"},
		},
		{
			name: "invalid enum",
			configs: []config{
				{env: "AAA", part: "aaa", def: "", valid: []string{"a1", "a2"}},
			},
			env:     map[string]string{"AAA": "not-a-valid-enum"},
			wantErr: true,
		},
		{
			name: "valid int",
			configs: []config{
				{env: "AAA", part: "aaa", def: 0},
			},
			env:  map[string]string{"AAA": "99"},
			want: map[string]string{"aaa": "99"},
		},
		{
			name: "invalid int",
			configs: []config{
				{env: "AAA", part: "aaa", def: 0},
			},
			env:     map[string]string{"AAA": "not-an-int"},
			wantErr: true,
		},
		{
			name: "no default int",
			configs: []config{
				{env: "AAA", part: "aaa", def: 0},
			},
		},
		{
			name: "default int",
			configs: []config{
				{env: "AAA", part: "aaa", def: 99},
			},
			want: map[string]string{"aaa": "99"},
		},
		{
			name: "valid duration",
			configs: []config{
				{env: "AAA", part: "aaa", def: time.Duration(0)},
			},
			env:  map[string]string{"AAA": "10s"},
			want: map[string]string{"aaa": "10s"},
		},
		{
			name: "invalid duration",
			configs: []config{
				{env: "AAA", part: "aaa", def: time.Duration(0)},
			},
			env:     map[string]string{"AAA": "not-a-duration"},
			wantErr: true,
		},
		{
			name: "default duration",
			configs: []config{
				{env: "AAA", part: "aaa", def: 5 * time.Second},
			},
			want: map[string]string{"aaa": "5s"},
		},
		{
			name: "no default duration",
			configs: []config{
				{env: "AAA", part: "aaa", def: time.Duration(0)},
			},
		},
		{
			name: "complex",
			configs: []config{
				{env: "AAA", part: "aaa", def: "", req: true},
				{env: "BBB", part: "bbb", def: 0},
				{env: "CCC", part: "ccc", def: time.Duration(0)},
				{env: "DDD", part: "ddd", def: "d_d"},
			},
			env:     map[string]string{"AAA": "a_a", "CCC": "10s"},
			want:    map[string]string{"aaa": "a_a", "ccc": "10s", "ddd": "d_d"},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			setupEnv(t, tc.env)

			got, err := dbValues(ctx, tc.configs, tc.env)

			if err != nil != tc.wantErr {
				t.Fatalf("processEnv got err %t, want err %t", err != nil, tc.wantErr)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func setupEnv(t *testing.T, env map[string]string) {
	t.Helper()

	old := map[string]string{}
	var clear []string

	for key, value := range env {
		if oldVal, ok := os.LookupEnv(key); ok {
			old[key] = oldVal
		} else {
			clear = append(clear, key)
		}

		if err := os.Setenv(key, value); err != nil {
			t.Fatalf("Failed to set %s=%s", key, value)
		}
	}

	t.Cleanup(func() {
		for key, value := range old {
			if err := os.Setenv(key, value); err != nil {
				t.Fatalf("Failed to reset %s=%s", key, value)
			}
		}
		for _, key := range clear {
			if err := os.Unsetenv(key); err != nil {
				t.Fatalf("Failed to unset %s", key)
			}
		}
	})
}
