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

package secretenv

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	testEnvVar = "VERY_FAKE_ENV_VAR"
)

type myEnv struct {
	Food string `envconfig:"VERY_FAKE_ENV_VAR"`
}

func TestResolveSecretNoSecretManager(t *testing.T) {
	ctx := context.Background()
	env := &myEnv{}
	os.Setenv(testEnvVar, "secret://yo/secret/value")

	expected := "environment contains secret values, but there is no secret manager configured"

	err := Process(ctx, "", env, nil)
	if err == nil {
		t.Errorf("expected error, got nil")
	} else if err.Error() != expected {
		t.Errorf("wrong error, want: `%v` got: %v", expected, err)
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

func TestResolveSecretEnv(t *testing.T) {
	cases := []struct {
		name        string
		varValue    string
		secretPath  string
		secretValue string
		secretError string
		want        string
		wantError   string
	}{
		{
			name:     "only set locally",
			varValue: "BAR",
			want:     "BAR",
		},
		{
			name:        "resolve from secret store",
			varValue:    "secret://value/for/test/1",
			secretPath:  "value/for/test/1",
			secretValue: "BAZ",
			want:        "BAZ",
		},
		{
			name:        "secret manager error",
			varValue:    "secret://value/for/test/2",
			secretPath:  "value/for/test/2",
			secretError: "secretive error",
			wantError:   "secretive error",
		},
	}

	for _, c := range cases {
		sm := NewTestSecretManager()

		if c.varValue == "" {
			os.Unsetenv(testEnvVar)
		} else {
			os.Setenv(testEnvVar, c.varValue)
		}
		if c.secretValue != "" {
			sm.values[c.secretPath] = c.secretValue
		}
		if c.secretError != "" {
			sm.errors[c.secretPath] = c.secretError
		}

		ctx := context.Background()

		env := &myEnv{}
		err := Process(ctx, "", env, sm)

		if c.wantError != "" && err == nil {
			t.Errorf("%v process want error: '%v' got: nil", c.name, c.wantError)
		} else if c.wantError != "" {
			if !strings.Contains(err.Error(), c.wantError) {
				t.Errorf("%v process want error containing: '%v', got: %w", c.name, c.wantError, err)
			}
		} else if c.want != env.Food {
			t.Errorf("%v process want '%v' got '%v'", c.name, c.want, env.Food)
		}
	}
}

type myEnvToFile struct {
	EnvToFile string `envconfig:"VERY_FAKE_ENV_VAR" secretfile:"true"`
}

func TestWriteSecretToFile(t *testing.T) {
	testVal := "YOU GOT IT"
	ctx := context.Background()

	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("%d", time.Now().Unix()))
	if err := os.Mkdir(tempDir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	os.Setenv("SECRETS_DIR", tempDir)
	os.Setenv("VERY_FAKE_ENV_VAR", "secret://path/to/secret?target=file")

	sm := NewTestSecretManager()
	sm.values["path/to/secret"] = testVal

	env := &myEnvToFile{}
	err := Process(ctx, "", env, sm)

	if err != nil {
		t.Fatalf("unable to process environment: %v", err)
	}

	if _, err := os.Stat(env.EnvToFile); err != nil {
		t.Errorf("expected %q to exist: %v", env.EnvToFile, err)
	}

	b, err := ioutil.ReadFile(env.EnvToFile)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := testVal, string(b); want != got {
		t.Errorf("expected %q to be %q", got, want)
	}
}
