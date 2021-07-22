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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/exportimport/model"
	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestAddGetUpdateExportConfig(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	exportImportDB := New(testDB)

	fromTime := time.Now().UTC().Add(-1 * time.Second)
	want := []*model.ExportImport{
		{
			IndexFile:  "https://myserver/exports/index.txt",
			ExportRoot: "https://myserver/",
			Region:     "US",
			From:       fromTime,
			Thru:       nil,
		},
		{
			IndexFile:  "https://myserver2/exports/index.txt",
			ExportRoot: "https://myserver2/",
			Region:     "US",
			Traveler:   true,
			From:       fromTime.Add(time.Hour),
			Thru:       nil,
		},
	}
	for _, w := range want {
		if err := exportImportDB.AddConfig(ctx, w); err != nil {
			t.Fatal(err)
		}
	}

	// list active configs
	{
		got, err := exportImportDB.ActiveConfigs(ctx)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(want[0:1], got, database.ApproxTime); diff != "" {
			t.Errorf("mismatch (-want, +got):\n%s", diff)
		}
	}

	// retrieve config by ID.
	{
		for i := range want {
			got, err := exportImportDB.GetConfig(ctx, want[i].ID)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(want[i], got, database.ApproxTime); diff != "" {
				t.Errorf("mismatch (-want, +got):\n%s", diff)
			}
		}
	}

	// list all configs
	{
		got, err := exportImportDB.ListConfigs(ctx)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(want, got, database.ApproxTime); diff != "" {
			t.Errorf("mismatch (-want, +got):\n%s", diff)
		}
	}

	// Change time one second config
	want[1].From = fromTime
	if err := exportImportDB.UpdateConfig(ctx, want[1]); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// list active configs, all are active now.
	{
		got, err := exportImportDB.ActiveConfigs(ctx)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(want, got, database.ApproxTime); diff != "" {
			t.Errorf("mismatch (-want, +got):\n%s", diff)
		}
	}
}

func TestAddImportFiles(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	exportImportDB := New(testDB)

	now := time.Now().UTC()
	config := model.ExportImport{
		IndexFile:  "https://mysever/exports/index.txt",
		ExportRoot: "https://myserver/",
		Region:     "US",
		From:       now,
		Thru:       nil,
	}
	if err := exportImportDB.AddConfig(ctx, &config); err != nil {
		t.Fatal(err)
	}

	filenames := []string{"a.zip", "b.zip", "c.zip"}

	if n, f, err := exportImportDB.CreateNewFilesAndFailOld(ctx, &config, filenames); err != nil {
		t.Fatal(err)
	} else if n != len(filenames) {
		t.Fatalf("incorrect number of files, want: %v, got: %v", len(filenames), n)
	} else if f != 0 {
		t.Fatalf("incorrect number of failed files, want: 0, got: %v", f)
	}

	lockDuration := 15 * time.Minute
	retryRate := time.Hour
	got, err := exportImportDB.GetOpenImportFiles(ctx, lockDuration, retryRate, &config)
	if err != nil {
		t.Fatal(err)
	}

	want := make([]*model.ImportFile, 0, len(filenames))
	for _, fname := range filenames {
		want = append(want,
			&model.ImportFile{
				ExportImportID: config.ID,
				ZipFilename:    fname,
				Status:         model.ImportFileOpen,
			})
	}

	opts := cmp.Options{cmpopts.IgnoreFields(model.ImportFile{}, "ID", "DiscoveredAt")}
	if diff := cmp.Diff(want, got, opts); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestLeaseAndCompleteImportFile(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	exportImportDB := New(testDB)

	lockDuration, retryRate := 2*time.Second, time.Hour
	now := time.Now().UTC()
	config := model.ExportImport{
		IndexFile:  "https://mysever/exports/index.txt",
		ExportRoot: "https://myserver/",
		Region:     "US",
		From:       now,
		Thru:       nil,
	}
	if err := exportImportDB.AddConfig(ctx, &config); err != nil {
		t.Fatal(err)
	}

	filenames := []string{"a.zip"}

	if n, f, err := exportImportDB.CreateNewFilesAndFailOld(ctx, &config, filenames); err != nil {
		t.Fatal(err)
	} else if n != len(filenames) {
		t.Fatalf("incorrect number of files, want: %v, got: %v", len(filenames), n)
	} else if f != 0 {
		t.Fatalf("incorrect number of failed files, want: 0, got: %v", f)
	}

	openFiles, err := exportImportDB.GetOpenImportFiles(ctx, lockDuration, retryRate, &config)
	if err != nil {
		t.Fatal(err)
	}
	if l := len(openFiles); l != 1 {
		t.Fatalf("didn't get expected files, want 1: got: %v", l)
	}

	testFile := openFiles[0]
	if err := exportImportDB.LeaseImportFile(ctx, lockDuration, testFile); err != nil {
		t.Fatalf("error locking file: %v", err)
	}

	if err := exportImportDB.LeaseImportFile(ctx, lockDuration, testFile); err == nil {
		t.Fatalf("no error trying to lock already locked file")
	}

	time.Sleep(2 * lockDuration)
	if err := exportImportDB.LeaseImportFile(ctx, lockDuration, testFile); err != nil {
		t.Fatalf("unable to lock file where lock has expired: %v", err)
	}

	if err := exportImportDB.CompleteImportFile(ctx, testFile, model.ImportFileComplete); err != nil {
		t.Fatalf("unable to complete import file: %v", err)
	}

	openFiles, err = exportImportDB.GetOpenImportFiles(ctx, lockDuration, retryRate, &config)
	if err != nil {
		t.Fatal(err)
	}
	if l := len(openFiles); l != 0 {
		t.Fatalf("wrong number of open files, want: 0, got: %v", l)
	}
}

func TestImportFilePublicKey(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	exportImportDB := New(testDB)

	now := time.Now().UTC()
	config := model.ExportImport{
		IndexFile:  "https://mysever/exports/index.txt",
		ExportRoot: "https://myserver/",
		Region:     "US",
		From:       now,
		Thru:       nil,
	}
	if err := exportImportDB.AddConfig(ctx, &config); err != nil {
		t.Fatal(err)
	}

	// Generate test ECDSA key pair.
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicKey := privateKey.Public()
	// Get the PEM for the public key.
	x509EncodedPub, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	pemEncodedPub := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: x509EncodedPub})
	pemPublicKey := string(pemEncodedPub)

	// Create import file public key.
	want := model.ImportFilePublicKey{
		ExportImportID: config.ID,
		KeyID:          "ghost",
		KeyVersion:     "v1",
		PublicKeyPEM:   pemPublicKey,
		From:           time.Now().UTC().Add(-1 * time.Hour),
		Thru:           nil,
	}

	if err := exportImportDB.AddImportFilePublicKey(ctx, &want); err != nil {
		t.Fatalf("error adding public key: %v", err)
	}

	got, err := exportImportDB.AllowedKeys(ctx, &config)
	if err != nil {
		t.Fatalf("error reading public keys: %v", err)
	}

	if diff := cmp.Diff(&want, got[0], database.ApproxTime); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

	if err := exportImportDB.ExpireImportFilePublicKey(ctx, &want); err != nil {
		t.Fatalf("failed to expire key: %v", err)
	}

	got, err = exportImportDB.AllowedKeys(ctx, &config)
	if err != nil {
		t.Fatalf("error reading public keys: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want 0 public keys, got: %v", got)
	}
}

func TestRetryToClose(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	exportImportDB := New(testDB)

	now := time.Now().UTC()
	config := model.ExportImport{
		IndexFile:  "https://mysever/exports/index.txt",
		ExportRoot: "https://myserver/",
		Region:     "US",
		From:       now,
		Thru:       nil,
	}
	if err := exportImportDB.AddConfig(ctx, &config); err != nil {
		t.Fatal(err)
	}

	filenames := []string{"a.zip", "b.zip", "c.zip"}

	if n, f, err := exportImportDB.CreateNewFilesAndFailOld(ctx, &config, filenames); err != nil {
		t.Fatalf("error creating files %v", err)
	} else if n != len(filenames) {
		t.Fatalf("error, only created %d files, expected %d", n, len(filenames))
	} else if f != 0 {
		t.Fatalf("error, failed %d files, expected 0", f)
	}

	// Make sure we get enough files.
	lockDuration := 15 * time.Minute
	retryRate := time.Hour
	if got, err := exportImportDB.GetOpenImportFiles(ctx, lockDuration, retryRate, &config); err != nil {
		t.Errorf("error getting open files; %v", err)
	} else if len(got) != len(filenames) {
		t.Errorf("got %d filenames, expected: %v", len(got), len(filenames))
	}

	if n, f, err := exportImportDB.CreateNewFilesAndFailOld(ctx, &config, []string{}); err != nil {
		t.Fatalf("error creating files")
	} else if n != 0 {
		t.Fatalf("error, only creating files, expected 0, got: %d", n)
	} else if f != 3 {
		t.Fatalf("error, got: %d failed, expected 3", f)
	}

	if got, err := exportImportDB.GetOpenImportFiles(ctx, lockDuration, retryRate, &config); err != nil {
		t.Errorf("error getting open files: %v", err)
	} else if len(got) != 0 {
		t.Errorf("expected all files deleted:, len = %d, expected 0", len(got))
	}

	if got, err := exportImportDB.GetAllImportFiles(ctx, lockDuration, &config); err != nil {
		t.Errorf("error getting open files; %v", err)
	} else if len(got) != len(filenames) {
		t.Errorf("got %d filenames, expected: %v", len(got), len(filenames))
	} else {
		for _, file := range got {
			if file.Status != model.ImportFileFailed {
				t.Errorf("input file status = %v, expected %v", file.Status, model.ImportFileFailed)
			}
		}
	}
}
