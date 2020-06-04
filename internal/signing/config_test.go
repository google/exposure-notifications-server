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

package signing_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/exposure-notifications-server/internal/envconfig"
	"github.com/google/exposure-notifications-server/internal/signing"
	"github.com/google/go-cmp/cmp"
)

func TestEnvconfigProcess(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    *signing.Config
		exp      *signing.Config
		lookuper envconfig.Lookuper
		err      error
	}{
		{
			name:     "nil",
			lookuper: envconfig.MapLookuper(map[string]string{}),
			err:      envconfig.ErrNotStruct,
		},
		{
			name:     "defaults",
			input:    &signing.Config{},
			exp:      signing.TestConfigDefaults(),
			lookuper: envconfig.MapLookuper(map[string]string{}),
		},
		{
			name:     "values",
			input:    &signing.Config{},
			exp:      signing.TestConfigValued(),
			lookuper: envconfig.MapLookuper(signing.TestConfigValues()),
		},
		{
			name:     "overrides",
			input:    signing.TestConfigOverridden(),
			exp:      signing.TestConfigOverridden(),
			lookuper: envconfig.MapLookuper(signing.TestConfigValues()),
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			if err := envconfig.ProcessWith(ctx, tc.input, tc.lookuper); !errors.Is(err, tc.err) {
				t.Fatalf("expected \n%#v\n to be \n%#v\n", err, tc.err)
			}

			if diff := cmp.Diff(tc.exp, tc.input); diff != "" {
				t.Fatalf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
