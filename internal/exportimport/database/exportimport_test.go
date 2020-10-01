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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/exportimport/model"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var (
	approxTime = cmp.Options{cmpopts.EquateApproxTime(time.Second)}
)

func TestAddGetUpdateExportConfig(t *testing.T) {
	t.Parallel()

	testDB := database.NewTestDatabase(t)
	ctx := context.Background()
	exportImportDB := New(testDB)

	fromTime := time.Now().UTC().Add(-1 * time.Second)
	want := []*model.ExportImport{
		{
			IndexFile:  "https://mysever/exports/index.txt",
			ExportRoot: "https://myserver/",
			Region:     "US",
			From:       fromTime,
			Thru:       nil,
		},
	}
	for _, w := range want {
		if err := exportImportDB.AddConfig(ctx, w); err != nil {
			t.Fatal(err)
		}
	}

	got, err := exportImportDB.ActiveConfigs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want, got, approxTime); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestAddImportFiles(t *testing.T) {
	t.Parallel()

	testDB := database.NewTestDatabase(t)
	ctx := context.Background()
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

	if n, err := exportImportDB.CreateFiles(ctx, &config, filenames); err != nil {
		t.Fatal(err)
	} else if n != len(filenames) {
		t.Fatalf("incorrect number of files, want: %v, got: %v", len(filenames), n)
	}

	lockDuration := 15 * time.Minute
	got, err := exportImportDB.GetOpenImportFiles(ctx, lockDuration, &config)
	if err != nil {
		t.Fatal(err)
	}

	var want []*model.ImportFile
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

	testDB := database.NewTestDatabase(t)
	ctx := context.Background()
	exportImportDB := New(testDB)

	lockDuration := 2 * time.Second
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

	if n, err := exportImportDB.CreateFiles(ctx, &config, filenames); err != nil {
		t.Fatal(err)
	} else if n != len(filenames) {
		t.Fatalf("incorrect number of files, want: %v, got: %v", len(filenames), n)
	}

	openFiles, err := exportImportDB.GetOpenImportFiles(ctx, lockDuration, &config)
	if err != nil {
		t.Fatal(err)
	}
	if l := len(openFiles); l != 1 {
		t.Fatalf("did't get expected files, want 1: got: %v", l)
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

	if err := exportImportDB.CompleteImportFile(ctx, testFile); err != nil {
		t.Fatalf("unable to complete import file: %v", err)
	}

	openFiles, err = exportImportDB.GetOpenImportFiles(ctx, lockDuration, &config)
	if err != nil {
		t.Fatal(err)
	}
	if l := len(openFiles); l != 0 {
		t.Fatalf("wrong number of open files, want: 0, got: %v", l)
	}
}

func TestImportFilePublicKey(t *testing.T) {
	t.Parallel()

	testDB := database.NewTestDatabase(t)
	ctx := context.Background()
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

	if diff := cmp.Diff(&want, got[0], approxTime); diff != "" {
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
