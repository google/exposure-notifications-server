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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	exportimportdb "github.com/google/exposure-notifications-server/internal/exportimport/database"
	"github.com/google/exposure-notifications-server/internal/exportimport/model"
	"github.com/google/exposure-notifications-server/internal/serverenv"
)

func TestImportingRetries(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	exportImportDB := exportimportdb.New(testDB)

	// Create a test http server.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()

	now := time.Now().UTC()
	eiConfig := model.ExportImport{
		IndexFile:  ts.URL + "/index.txt",
		ExportRoot: ts.URL,
		Region:     "US",
		From:       now,
		Thru:       nil,
	}
	if err := exportImportDB.AddConfig(ctx, &eiConfig); err != nil {
		t.Fatal(err)
	}

	filenames := []string{"a.zip"}

	if n, f, err := exportImportDB.CreateNewFilesAndFailOld(ctx, &eiConfig, filenames); err != nil {
		t.Fatal(err)
	} else if n != len(filenames) {
		t.Fatalf("incorrect number of files, want: %v, got: %v", len(filenames), n)
	} else if f != 0 {
		t.Fatalf("incorrect number of failed files, want: 0, got: %v", f)
	}

	// Create the server.
	config := &Config{}
	env := serverenv.New(ctx, serverenv.WithDatabase(testDB))
	s, err := NewServer(config, env)
	if err != nil {
		t.Fatalf("error creating server: %v", err)
	}

	// Run the importer a bunch.
	for i := 0; i < 10; i++ {
		if err := s.runImport(ctx, &eiConfig); err != nil {
			if i != 0 {
				t.Errorf("expected no error %v", err)
			}
		}
	}
}
