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

package secrets

import (
	"testing"

	"github.com/google/exposure-notifications-server/internal/project"
)

func TestJSONExpander_GetSecretValue(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	cases := []struct {
		testName      string
		secretName    string
		secretValue   string
		expectedValue string
		err           bool
	}{
		{
			testName:      "simple name and simple value",
			secretName:    "psqlcreds",
			secretValue:   "abc-123",
			expectedValue: "abc-123",
			err:           false,
		},
		{
			testName:      "simple name and json value",
			secretName:    "psqlcreds",
			secretValue:   "{\"username\":\"gandalf\", \"password\":\"abc-123\"}",
			expectedValue: "{\"username\":\"gandalf\", \"password\":\"abc-123\"}",
			err:           false,
		},
		{
			testName:      "unknown expansion key and json value",
			secretName:    "psqlcreds.unknown",
			secretValue:   "{\"username\":\"gandalf\", \"password\":\"abc-123\"}",
			expectedValue: "",
			err:           true,
		},
		{
			testName:      "json expansion name and json value",
			secretName:    "psqlcreds.username",
			secretValue:   "{\"username\":\"gandalf\", \"password\":\"abc-123\"}",
			expectedValue: "gandalf",
			err:           false,
		},
		{
			testName:      "json expansion name second value and json value",
			secretName:    "psqlcreds.password",
			secretValue:   "{\"username\":\"gandalf\", \"password\":\"abc-123\"}",
			expectedValue: "abc-123",
			err:           false,
		},
		{
			testName:      "json expansion name and simple value",
			secretName:    "psqlcreds.password",
			secretValue:   "abc-123",
			expectedValue: "",
			err:           true,
		},
		{
			testName:      "simple name and invalid json",
			secretName:    "psqlcreds",
			secretValue:   "{\"invalid!\"",
			expectedValue: "{\"invalid!\"",
			err:           false,
		},
		{
			testName:      "json expansion name and invalid json",
			secretName:    "psqlcreds.username",
			secretValue:   "{\"invalid!\"",
			expectedValue: "",
			err:           true,
		},
		{
			testName:      "json expansion name and non string in json value",
			secretName:    "psqlcreds.username",
			secretValue:   "{\"username\":5}",
			expectedValue: "",
			err:           true,
		},
		{
			testName:      "nested json expansion name",
			secretName:    "psqlcreds.creds.username",
			secretValue:   "{\"creds\":{\"username\":\"gandalf\"}}",
			expectedValue: "gandalf",
			err:           false,
		},
		{
			testName:      "nested json unknown key",
			secretName:    "psqlcreds.creds.password",
			secretValue:   "{\"creds\":{\"username\":\"gandalf\"}}",
			expectedValue: "",
			err:           true,
		},
		{
			testName:      "nested json expansion name and non string in json value",
			secretName:    "psqlcreds.creds.username",
			secretValue:   "{\"creds\":{\"username\":5}}",
			expectedValue: "",
			err:           true,
		},
	}
	for _, tc := range cases {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			testSM := &testSecretManager{}
			testSM.value = "test"

			sm, err := WrapJSONExpander(ctx, testSM)
			if err != nil {
				t.Fatal(err)
			}

			testSM.value = tc.secretValue
			actualValue, err := sm.GetSecretValue(ctx, tc.secretName)
			if err != nil {
				if !tc.err {
					t.Errorf("got error: %w, did not expect one", err)
				}
			}
			if tc.err && err == nil {
				t.Errorf("expected to error, but did not, actualValue: %s", actualValue)
			}
			if tc.expectedValue != actualValue {
				t.Errorf("expected %s, got %s", tc.expectedValue, actualValue)
			}
		})
	}
}
