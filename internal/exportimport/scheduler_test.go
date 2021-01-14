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

package exportimport

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	exportimportdb "github.com/google/exposure-notifications-server/internal/exportimport/database"
	"github.com/google/exposure-notifications-server/internal/exportimport/model"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestSyncFileFromIndexErrorsInExportRoot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	exportImportDB := exportimportdb.New(testDB)

	fromTime := time.Now().UTC().Add(-1 * time.Second)
	config := &model.ExportImport{
		IndexFile:  "index.txt",
		ExportRoot: "%zzzzz",
		Region:     "US",
		From:       fromTime,
		Thru:       nil,
	}
	if err := exportImportDB.AddConfig(ctx, config); err != nil {
		t.Fatal(err)
	}

	// test data ensures that URL parsing strips extra slashes.
	index := strings.Join([]string{"a.zip", "/b.zip", "//c.zip", ""}, "\n")

	if _, _, err := syncFilesFromIndex(ctx, exportImportDB, config, index); err == nil {
		t.Fatalf("expected error")
	} else if !strings.Contains(err.Error(), "invalid URL escape") {
		t.Fatalf("wrong error, wanted: invalid URL escape, got: %v", err)
	}
}

func TestSyncFilenameShapes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	exportImportDB := exportimportdb.New(testDB)

	fromTime := time.Now().UTC().Add(-1 * time.Second)

	cases := []struct {
		name   string
		config *model.ExportImport
		index  []string
		want   []string
	}{
		{
			name: "shallow_root_no_path",
			config: &model.ExportImport{
				IndexFile:  "https://cdn.example.com/index.txt",
				ExportRoot: "https://cdn.example.com",
				Region:     "US",
				From:       fromTime,
			},
			index: []string{
				"/export-a.zip",
				"/export-b.zip",
			},
			want: []string{
				"https://cdn.example.com/export-a.zip",
				"https://cdn.example.com/export-b.zip",
			},
		},
		{
			name: "shallow_root_no_path_trailing_slash",
			config: &model.ExportImport{
				IndexFile:  "https://cdn.example.com/index.txt",
				ExportRoot: "https://cdn.example.com/",
				Region:     "US",
				From:       fromTime,
			},
			index: []string{
				"/export-a.zip",
				"/export-b.zip",
			},
			want: []string{
				"https://cdn.example.com/export-a.zip",
				"https://cdn.example.com/export-b.zip",
			},
		},
		{
			name: "nested_paths",
			config: &model.ExportImport{
				IndexFile:  "https://cdn.example.com/folder/region-us/index.txt",
				ExportRoot: "https://cdn.example.com/folder/",
				Region:     "US",
				From:       fromTime,
			},
			index: []string{
				"region-us/export-a.zip",
				"region-us/export-b.zip",
			},
			want: []string{
				"https://cdn.example.com/folder/region-us/export-a.zip",
				"https://cdn.example.com/folder/region-us/export-b.zip",
			},
		},
		{
			name: "nested_paths_missing_slash",
			config: &model.ExportImport{
				IndexFile:  "https://cdn.example.com/folder/region-us/index.txt",
				ExportRoot: "https://cdn.example.com/folder",
				Region:     "US",
				From:       fromTime,
			},
			index: []string{
				"region-us/export-a.zip",
				"region-us/export-b.zip",
			},
			want: []string{
				"https://cdn.example.com/folder/region-us/export-a.zip",
				"https://cdn.example.com/folder/region-us/export-b.zip",
			},
		},
		{
			name: "deeply_nested_paths",
			config: &model.ExportImport{
				IndexFile:  "https://cdn.example.com/folder/a/b/c/region-us/index.txt",
				ExportRoot: "https://cdn.example.com/folder/a/b",
				Region:     "US",
				From:       fromTime,
			},
			index: []string{
				"c/region-us/export-a.zip",
				"c/region-us/export-b.zip",
			},
			want: []string{
				"https://cdn.example.com/folder/a/b/c/region-us/export-a.zip",
				"https://cdn.example.com/folder/a/b/c/region-us/export-b.zip",
			},
		},
		{
			name: "one_folder",
			config: &model.ExportImport{
				IndexFile:  "https://cdn.example.com/region-us/index.txt",
				ExportRoot: "https://cdn.example.com/",
				Region:     "US",
				From:       fromTime,
			},
			index: []string{
				"region-us/export-a.zip",
				"region-us/export-b.zip",
			},
			want: []string{
				"https://cdn.example.com/region-us/export-a.zip",
				"https://cdn.example.com/region-us/export-b.zip",
			},
		},
		{
			name: "one_folder_without_slash",
			config: &model.ExportImport{
				IndexFile:  "https://cdn.example.com/region-us/index.txt",
				ExportRoot: "https://cdn.example.com",
				Region:     "US",
				From:       fromTime,
			},
			index: []string{
				"region-us/export-a.zip",
				"region-us/export-b.zip",
			},
			want: []string{
				"https://cdn.example.com/region-us/export-a.zip",
				"https://cdn.example.com/region-us/export-b.zip",
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			config := tc.config
			if err := exportImportDB.AddConfig(ctx, config); err != nil {
				t.Fatal(err)
			}

			index := strings.Join(tc.index, "\n")

			wantN := len(tc.want)
			if n, f, err := syncFilesFromIndex(ctx, exportImportDB, config, index); err != nil {
				t.Fatal(err)
			} else if n != wantN {
				t.Fatalf("wanted sync result of %d new files, got: %d", wantN, n)
			} else if f != 0 {
				t.Fatalf("wanted sync result of 0 failed files, got: %d", f)
			}

			files, err := exportImportDB.GetOpenImportFiles(ctx, time.Second, time.Hour, config)
			if err != nil {
				t.Fatal(err)
			}

			got := make([]string, len(files))
			for i, f := range files {
				got[i] = f.ZipFilename
			}

			options := cmp.Options{
				cmpopts.SortSlices(func(a, b string) bool {
					return strings.Compare(a, b) <= 0
				}),
			}
			if diff := cmp.Diff(tc.want, got, options); diff != "" {
				t.Errorf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestSyncFileFromIndex(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	exportImportDB := exportimportdb.New(testDB)

	fromTime := time.Now().UTC().Add(-1 * time.Second)

	cases := []struct {
		name       string
		exportRoot string
	}{
		{
			name:       "with_slash",
			exportRoot: "https://myserver/",
		},
		{
			name:       "without_slash",
			exportRoot: "https://myserver",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			config := &model.ExportImport{
				IndexFile:  "https://mysever/exports/index.txt",
				ExportRoot: tc.exportRoot,
				Region:     "US",
				From:       fromTime,
				Thru:       nil,
			}
			if err := exportImportDB.AddConfig(ctx, config); err != nil {
				t.Fatal(err)
			}

			// test data ensures that URL parsing strips extra slashes.
			index := strings.Join([]string{"a.zip", "/b.zip", "//c.zip", ""}, "\n")

			if n, f, err := syncFilesFromIndex(ctx, exportImportDB, config, index); err != nil {
				t.Fatal(err)
			} else if n != 3 {
				t.Fatalf("wanted sync result of 3, got: %d", n)
			} else if f != 0 {
				t.Fatalf("wanted sync result of 0 failed, got: %d", f)
			}

			files, err := exportImportDB.GetOpenImportFiles(ctx, time.Second, time.Hour, config)
			if err != nil {
				t.Fatal(err)
			}

			want := []*model.ImportFile{
				{
					ExportImportID: config.ID,
					ZipFilename:    fmt.Sprintf("https://myserver/%s", "a.zip"),
					Status:         model.ImportFileOpen,
				},
				{
					ExportImportID: config.ID,
					ZipFilename:    fmt.Sprintf("https://myserver/%s", "b.zip"),
					Status:         model.ImportFileOpen,
				},
				{
					ExportImportID: config.ID,
					ZipFilename:    fmt.Sprintf("https://myserver/%s", "c.zip"),
					Status:         model.ImportFileOpen,
				},
			}

			options := cmp.Options{
				cmpopts.IgnoreFields(model.ImportFile{}, "ID", "DiscoveredAt", "ProcessedAt"),
				cmpopts.SortSlices(func(a, b *model.ImportFile) bool {
					return strings.Compare(a.ZipFilename, b.ZipFilename) <= 0
				}),
			}
			if diff := cmp.Diff(want, files, options); diff != "" {
				t.Errorf("mismatch (-want, +got):\n%s", diff)
			}

			// shift the index
			index = strings.Join([]string{"/b.zip", "/c.zip", "/d.zip"}, "\n")

			if n, f, err := syncFilesFromIndex(ctx, exportImportDB, config, index); err != nil {
				t.Fatal(err)
			} else if n != 1 {
				t.Fatalf("wanted sync result of 1, got: %d", n)
			} else if f != 1 {
				t.Fatalf("wanted sync result of 1 failed, got: %d", f)
			}

			files, err = exportImportDB.GetOpenImportFiles(ctx, time.Second, time.Hour, config)
			if err != nil {
				t.Fatal(err)
			}

			want = []*model.ImportFile{
				{
					ExportImportID: config.ID,
					ZipFilename:    fmt.Sprintf("https://myserver/%s", "b.zip"),
					Status:         model.ImportFileOpen,
				},
				{
					ExportImportID: config.ID,
					ZipFilename:    fmt.Sprintf("https://myserver/%s", "c.zip"),
					Status:         model.ImportFileOpen,
				},
				{
					ExportImportID: config.ID,
					ZipFilename:    fmt.Sprintf("https://myserver/%s", "d.zip"),
					Status:         model.ImportFileOpen,
				},
			}

			if diff := cmp.Diff(want, files, options); diff != "" {
				t.Errorf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
