// Copyright 2020 the Exposure Notifications Server authors
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
	"testing"

	"github.com/google/exposure-notifications-server/internal/mirror/model"
	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestMirror_Lifecycle(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
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

	// List
	mirrors, err := mirrorDB.Mirrors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(mirrors) < 1 {
		t.Fatal("no mirrors")
	}

	ignore := cmpopts.IgnoreUnexported(model.Mirror{})
	if diff := cmp.Diff(want, mirrors, ignore); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

	// Get and update
	{
		mirror, err := mirrorDB.GetMirror(ctx, mirrors[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		if mirror.ID != 1 {
			t.Fatalf("expected %d to be 1", mirror.ID)
		}

		mirror.IndexFile = "foo"
		if err := mirrorDB.UpdateMirror(ctx, mirror); err != nil {
			t.Fatal(err)
		}
		mirror, err = mirrorDB.GetMirror(ctx, mirrors[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := mirror.IndexFile, "foo"; got != want {
			t.Fatalf("expected %q to be %q", got, want)
		}
	}

	for _, w := range want {
		if err := mirrorDB.DeleteMirror(ctx, w); err != nil {
			t.Fatal(err)
		}
	}

	got, err := mirrorDB.Mirrors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got back deleted mirrors")
	}
}

func TestFileLifecycle(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
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

	filenames := []*SyncFile{
		{RemoteFile: "a", LocalFile: "a"},
		{RemoteFile: "b", LocalFile: "b"},
		{RemoteFile: "c", LocalFile: "c"},
	}
	if err := mirrorDB.SaveFiles(ctx, mirror.ID, filenames); err != nil {
		t.Fatal(err)
	}

	want := make([]*model.MirrorFile, len(filenames))
	for i, fname := range filenames {
		want[i] = &model.MirrorFile{
			MirrorID: mirror.ID,
			Filename: fname.RemoteFile,
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
	filenames = []*SyncFile{
		{RemoteFile: "c", LocalFile: "c"},
		{RemoteFile: "d", LocalFile: "d"},
		{RemoteFile: "e", LocalFile: "e"},
	}

	if err := mirrorDB.SaveFiles(ctx, mirror.ID, filenames); err != nil {
		t.Fatal(err)
	}

	want = make([]*model.MirrorFile, len(filenames))
	for i, fname := range filenames {
		want[i] = &model.MirrorFile{
			MirrorID: mirror.ID,
			Filename: fname.RemoteFile,
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

func TestFileLifecycleWithRewrite(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
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

	filenames := []*SyncFile{
		{RemoteFile: "a.zip", LocalFile: "1234-0001.zip"},
		{RemoteFile: "b.zip", LocalFile: "2345-0001.zip"},
		{RemoteFile: "c.zip", LocalFile: "3456-0001.zip"},
	}
	if err := mirrorDB.SaveFiles(ctx, mirror.ID, filenames); err != nil {
		t.Fatal(err)
	}

	want := make([]*model.MirrorFile, len(filenames))
	for i, fname := range filenames {
		localFname := fname.LocalFile
		want[i] = &model.MirrorFile{
			MirrorID:      mirror.ID,
			Filename:      fname.RemoteFile,
			LocalFilename: &localFname,
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
	filenames = []*SyncFile{
		{RemoteFile: "c.zip", LocalFile: "3456-0001.zip"},
		{RemoteFile: "d.zip", LocalFile: "4567-0001.zip"},
		{RemoteFile: "e.zip", LocalFile: "25678-0001.zip"},
	}

	if err := mirrorDB.SaveFiles(ctx, mirror.ID, filenames); err != nil {
		t.Fatal(err)
	}

	want = make([]*model.MirrorFile, len(filenames))
	for i, fname := range filenames {
		localFname := fname.LocalFile
		want[i] = &model.MirrorFile{
			MirrorID:      mirror.ID,
			Filename:      fname.RemoteFile,
			LocalFilename: &localFname,
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
