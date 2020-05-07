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

package serverenv

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseDurationEnv(t *testing.T) {
	ctx := context.Background()
	const varName = "PARSE_DURATION_TEST"
	const defaultValue = 17 * time.Second
	for _, test := range []struct {
		val  string
		want time.Duration
	}{
		{"", defaultValue},
		{"bad", defaultValue},
		{"250ms", 250 * time.Millisecond},
	} {
		os.Setenv(varName, test.val)
		got := ParseDuration(ctx, varName, defaultValue)
		if got != test.want {
			t.Errorf("%q: got %v, want %v", test.val, got, test.want)
		}
	}
}

func TestServerEnv(t *testing.T) {
	ctx := context.Background()
	os.Setenv(portEnvVar, "4000")
	env := New(ctx)

	if port := env.Port(); port != "4000" {
		t.Errorf("env.Port got %v want 4000", port)
	}
}

type TestSecretManager struct {
	values map[string]string
	errors map[string]string
}

func NewTestSecretManager() *TestSecretManager {
	return &TestSecretManager{
		values: make(map[string]string),
		errors: make(map[string]string),
	}
}

func (s *TestSecretManager) GetSecretValue(ctx context.Context, name string) (string, error) {
	if v, ok := s.errors[name]; ok {
		return "", fmt.Errorf(v)
	}
	if v, ok := s.values[name]; ok {
		return v, nil
	}
	return "", nil
}

func TestResolveSecretNoSecretManager(t *testing.T) {
	ctx := context.Background()
	env := New(ctx)

	os.Setenv("MOOSE", "MUFFIN")
	resolve, err := env.ResolveSecretEnv(ctx, "MOOSE")
	if err != nil {
		t.Errorf("env.ResolveSecretEnv: unexpected error %w", err)
	}
	if resolve != "MUFFIN" {
		t.Errorf("env.ResolveSecretEnv: want: MOOSE, got: %v", resolve)
	}
}

func TestResolveSecretEnv(t *testing.T) {
	cases := []struct {
		name        string
		varName     string
		varValue    string
		secretValue string
		secretError string
		want        string
		wantError   string
		override    string
	}{
		{
			name:     "only set locally",
			varName:  "FOO",
			varValue: "BAR",
			want:     "BAR",
		},
		{
			name:        "resolve from secret store",
			varName:     "FOO",
			varValue:    "BAR",
			secretValue: "BAZ",
			want:        "BAZ",
		},
		{
			name:        "secret manager error",
			varName:     "FOO",
			varValue:    "BAR",
			secretError: "secretive error",
			wantError:   "secretive error",
		},
		{
			name:        "override",
			varName:     "TST",
			secretValue: "ORIGINAL",
			want:        "ORVALUE",
			override:    "ORVALUE",
		},
	}

	for _, c := range cases {
		sm := NewTestSecretManager()

		if c.varValue == "" {
			os.Unsetenv(c.varName)
		} else {
			os.Setenv(c.varName, c.varValue)
		}
		if c.secretValue != "" || c.secretError != "" {
			os.Setenv(c.varName+SecretPostfix, c.varName)
		}
		if c.secretValue != "" {
			sm.values[c.varName] = c.secretValue
		}
		if c.secretError != "" {
			sm.errors[c.varName] = c.secretError
		}

		ctx := context.Background()
		env := New(ctx, WithSecretManager(sm))

		if c.override != "" {
			env.Set(c.varName, c.override)
		}

		resolved, err := env.ResolveSecretEnv(ctx, c.varName)
		if c.wantError != "" && err == nil {
			t.Errorf("%v env.ResolveSecretEnv want error: '%v' got: nil", c.name, c.wantError)
		} else if c.wantError != "" && !strings.Contains(err.Error(), c.wantError) {
			t.Errorf("%v env.ResolveSecretEnv want error containing: '%v', got: %w", c.name, c.wantError, err)
		} else if c.want != resolved {
			t.Errorf("%v env.ResolveSecretEnv want '%v' got '%v'", c.name, c.want, resolved)
		}
	}
}
