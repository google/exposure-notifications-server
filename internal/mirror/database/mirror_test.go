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
	"testing"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/mirror/model"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestAddListDeleteMirror(t *testing.T) {
	t.Parallel()

	testDB := database.NewTestDatabase(t)
	ctx := context.Background()
	mirrorDB := New(testDB)

	want := []*model.Mirror{
		{
			IndexFile:          "https://mysever/exports/index.txt",
			ExportRoot:         "https://myserver/",
			CloudStorageBucket: "b1",
			FilenameRoot:       "/storage/is/awesome/",
		},
		{
			IndexFile:          "https://mysever2/exports/index.txt",
			ExportRoot:         "https://myserver2/",
			CloudStorageBucket: "b2",
			FilenameRoot:       "/storage/is/awesome2/",
		},
	}
	for _, w := range want {
		if err := mirrorDB.AddMirror(ctx, w); err != nil {
			t.Fatal(err)
		}
	}

	got, err := mirrorDB.Mirrors(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

	for _, w := range want {
		if err := mirrorDB.DeleteMirror(ctx, w); err != nil {
			t.Fatal(err)
		}
	}

	got, err = mirrorDB.Mirrors(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 0 {
		t.Fatalf("got back deleted mirrors")
	}
}

func TestFileLifecycle(t *testing.T) {
	t.Parallel()

	testDB := database.NewTestDatabase(t)
	ctx := context.Background()
	mirrorDB := New(testDB)

	mirror := model.Mirror{
		IndexFile:          "https://mysever/exports/index.txt",
		ExportRoot:         "https://myserver/",
		CloudStorageBucket: "b1",
		FilenameRoot:       "/storage/is/awesome/",
	}

	if err := mirrorDB.AddMirror(ctx, &mirror); err != nil {
		t.Fatal(err)
	}

	filenames := []string{"a", "b", "c"}
	if err := mirrorDB.SaveFiles(ctx, mirror.ID, filenames); err != nil {
		t.Fatal(err)
	}

	want := make([]*model.MirrorFile, len(filenames))
	for i, fname := range filenames {
		want[i] = &model.MirrorFile{
			MirrorID: mirror.ID,
			Filename: fname,
		}
	}

	got, err := mirrorDB.ListFiles(ctx, mirror.ID)
	if err != nil {
		t.Fatal(err)
	}

	sorter := cmpopts.SortSlices(
		func(a, b *model.MirrorFile) bool {
			return a.Filename < b.Filename
		})
	if diff := cmp.Diff(want, got, sorter); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

	// Change the files
	filenames = []string{"c", "d", "e"}

	if err := mirrorDB.SaveFiles(ctx, mirror.ID, filenames); err != nil {
		t.Fatal(err)
	}

	want = make([]*model.MirrorFile, len(filenames))
	for i, fname := range filenames {
		want[i] = &model.MirrorFile{
			MirrorID: mirror.ID,
			Filename: fname,
		}
	}

	got, err = mirrorDB.ListFiles(ctx, mirror.ID)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want, got, sorter); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}
