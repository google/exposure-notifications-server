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
	"context"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestResolver(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	sm, err := NewInMemoryFromMap(ctx, map[string]string{
		"my-secret1": "value1",
		"my-secret2": "value2",
	})
	if err != nil {
		t.Fatal(err)
	}

	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(tmpdir); err != nil {
			t.Fatal(err)
		}
	})

	cases := []struct {
		name  string
		dir   string
		key   string
		value string
		exp   string
		err   bool
	}{
		{
			name:  "bad_dir_no_file",
			dir:   "/totally/not/a/valid/dir",
			key:   "foo",
			value: "secret://my-secret1",
			exp:   "value1",
		},
		{
			name:  "to_file",
			dir:   tmpdir,
			key:   "foo",
			value: "secret://my-secret1?target=file",
			exp:   tmpdir,
		},
		{
			name:  "not_a_secret",
			key:   "foo",
			value: "not-a-secret",
			exp:   "not-a-secret",
		},
		{
			name:  "no_exist",
			key:   "foo",
			value: "secret://not-a-secret",
			err:   true,
		},
		{
			name:  "multi",
			key:   "foo",
			value: "secret://my-secret1,secret://my-secret2",
			exp:   "value1,value2",
		},
		{
			name:  "multi_mixed",
			key:   "foo",
			value: "secret://my-secret1,secret://my-secret2,value3",
			exp:   "value1,value2,value3",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			config := &Config{SecretsDir: tc.dir}
			result, err := Resolver(sm, config)(ctx, tc.key, tc.value)
			if (err != nil) != tc.err {
				t.Fatal(err)
			}

			if got, want := result, tc.exp; !strings.HasPrefix(got, want) {
				t.Errorf("expected %v to be prefixed with %v", got, want)
			}
		})
	}
}
