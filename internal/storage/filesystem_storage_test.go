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

package storage

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestFilesystemStorage_CreateObject(t *testing.T) {
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmp) })

	cases := []struct {
		name     string
		folder   string
		filepath string
		contents []byte
		err      bool
	}{
		{
			name:     "default",
			folder:   tmp,
			filepath: "myfile",
			contents: []byte("contents"),
		},
		{
			name:     "bad_path",
			folder:   "/path/that/definitely/doesnt/exist",
			filepath: "myfile",
			contents: []byte("contents"),
			err:      true,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			storage, err := NewFilesystemStorage(ctx)
			if err != nil {
				t.Fatal(err)
			}

			err = storage.CreateObject(ctx, tc.folder, tc.filepath, tc.contents, false)
			if (err != nil) != tc.err {
				t.Fatal(err)
			}

			if !tc.err {
				contents, err := ioutil.ReadFile(filepath.Join(tc.folder, tc.filepath))
				if err != nil {
					t.Fatal(err)
				}

				if !bytes.Equal(contents, tc.contents) {
					t.Errorf("expected %q to be %q ", contents, tc.contents)
				}
			}
		})
	}
}

func TestFilesystemStorage_DeleteObject(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		folder   string
		filepath string
	}{
		{
			name:     "default",
			folder:   filepath.Dir(f.Name()),
			filepath: filepath.Base(f.Name()),
		},
		{
			name:     "not_exist",
			folder:   filepath.Dir(f.Name()),
			filepath: "not-exist",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			storage, err := NewFilesystemStorage(ctx)
			if err != nil {
				t.Fatal(err)
			}

			if err = storage.DeleteObject(ctx, tc.folder, tc.filepath); err != nil {
				t.Fatal(err)
			}
		})
	}
}
