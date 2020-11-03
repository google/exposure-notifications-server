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

	"github.com/google/exposure-notifications-server/internal/database"
	exportimportdb "github.com/google/exposure-notifications-server/internal/exportimport/database"
	"github.com/google/exposure-notifications-server/internal/exportimport/model"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestSyncFileFromIndex(t *testing.T) {
	t.Parallel()

	testDB := database.NewTestDatabase(t)
	ctx := context.Background()
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
		t.Run(tc.name, func(t *testing.T) {

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

			// test data ensures that URL parsing stripps extra slashes.
			index := strings.Join([]string{"a.zip", "/b.zip", "//c.zip", ""}, "\n")

			if n, err := syncFilesFromIndex(ctx, exportImportDB, config, index); err != nil {
				t.Fatal(err)
			} else if n != 3 {
				t.Fatalf("wanted sync result of 3, got: %d", n)
			}

			files, err := exportImportDB.GetOpenImportFiles(ctx, time.Second, config)
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

			if n, err := syncFilesFromIndex(ctx, exportImportDB, config, index); err != nil {
				t.Fatal(err)
			} else if n != 1 {
				t.Fatalf("wanted sync result of 1, got: %d", n)
			}

			files, err = exportImportDB.GetOpenImportFiles(ctx, time.Second, config)
			if err != nil {
				t.Fatal(err)
			}

			want = []*model.ImportFile{
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
