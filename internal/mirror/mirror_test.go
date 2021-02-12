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

package mirror

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	mirrordatabase "github.com/google/exposure-notifications-server/internal/mirror/database"
	mirrormodel "github.com/google/exposure-notifications-server/internal/mirror/model"
	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/google/go-cmp/cmp"
	"github.com/gorilla/mux"
	"github.com/sethvargo/go-envconfig"
)

var testDatabaseInstance *database.TestInstance

func TestMain(m *testing.M) {
	testDatabaseInstance = database.MustTestInstance()
	defer testDatabaseInstance.MustClose()
	m.Run()
}

func TestServer_ProcessMirror(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	zipFileContents := "data data data"

	cases := []struct {
		name        string
		indexFiles  []string
		mirrorFiles []*mirrordatabase.SyncFile

		// Note: ID, IndexFile are populated by the test.
		mirror *mirrormodel.Mirror

		// expBlobstoreDeleted is a list of files in the blobstore to expect to be
		// deleted. The first item is the bucket name, the second is the object
		// name.
		expBlobstoreDeleted [][2]string

		// expBlobstoreAdded is the list of files in the in the blobstore that
		// should exist after the run has completed. The first item is the bucket
		// name, the second is the object name.
		expBlobstoreAdded [][2]string
	}{
		{
			name: "empty",
			indexFiles: []string{
				"1605818705-1605819005-00001.zip",
				"1605818705-1605819005-00002.zip",

				// Not currently in local copy, should be mirrored.
				"1605819705-1605820005-00001.zip",
			},
			mirrorFiles: []*mirrordatabase.SyncFile{
				{RemoteFile: "1605818705-1605819005-00001.zip"},
				{RemoteFile: "1605818705-1605819005-00002.zip"},

				// Old files that are no longer in the index (see indexFiles above)
				{RemoteFile: "1605818705-1605819005-00003.zip"},
				{RemoteFile: "1605818705-1605819005-00004.zip"},
				{RemoteFile: "1605818705-1605819005-00005.zip"},
			},
			mirror: &mirrormodel.Mirror{
				ExportRoot: "",

				CloudStorageBucket: "",
				FilenameRoot:       "",
			},
			expBlobstoreDeleted: [][2]string{
				{"", "1605818705-1605819005-00003.zip"},
				{"", "1605818705-1605819005-00004.zip"},
				{"", "1605818705-1605819005-00005.zip"},
			},
			expBlobstoreAdded: [][2]string{
				{"", "1605819705-1605820005-00001.zip"},
			},
		},
		{
			name: "configured",
			indexFiles: []string{
				"us/1605818705-1605819005-00001.zip",
				"us/1605818705-1605819005-00002.zip",

				// Not currently in local copy, should be mirrored.
				"us/1605819705-1605820005-00001.zip",
			},
			mirrorFiles: []*mirrordatabase.SyncFile{
				{RemoteFile: "1605818705-1605819005-00001.zip"},
				{RemoteFile: "1605818705-1605819005-00002.zip"},

				// Old files that are no longer in the index (see indexFiles above)
				{RemoteFile: "1605818705-1605819005-00003.zip"},
				{RemoteFile: "1605818705-1605819005-00004.zip"},
				{RemoteFile: "1605818705-1605819005-00005.zip"},
			},
			mirror: &mirrormodel.Mirror{
				ExportRoot: "us",

				CloudStorageBucket: "bucket",
				FilenameRoot:       "my/other/path",
			},
			expBlobstoreDeleted: [][2]string{
				{"bucket", "my/other/path/1605818705-1605819005-00003.zip"},
				{"bucket", "my/other/path/1605818705-1605819005-00004.zip"},
				{"bucket", "my/other/path/1605818705-1605819005-00005.zip"},
			},
			expBlobstoreAdded: [][2]string{
				{"bucket", "my/other/path/1605819705-1605820005-00001.zip"},
			},
		},
		{
			name: "rewrite",
			indexFiles: []string{
				"1605818705-1605819005-00001.zip",
				"1605818705-1605819005-00002.zip",
			},
			mirrorFiles: []*mirrordatabase.SyncFile{},
			mirror: &mirrormodel.Mirror{
				// This isn't super great, but we can't really test UUID or timestamp
				// behavior predictably. This will just rewrite all the files to the
				// same name, but we can at least assert that the rewrite happened.
				FilenameRewrite: stringPtr("xx-[test].zip"),
			},
			expBlobstoreAdded: [][2]string{
				{"", "xx-TEST.zip"},
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			testDB, _ := testDatabaseInstance.NewDatabase(t)
			mirrorDB := mirrordatabase.New(testDB)
			testBlobstore, err := storage.NewMemory(ctx, &storage.Config{})
			if err != nil {
				t.Fatal(err)
			}

			env := serverenv.New(ctx,
				serverenv.WithDatabase(testDB),
				serverenv.WithBlobStorage(testBlobstore),
			)

			var config Config
			if err := envconfig.ProcessWith(ctx, &config, envconfig.MapLookuper(nil)); err != nil {
				t.Fatal(err)
			}

			s, err := NewServer(&config, env)
			if err != nil {
				t.Fatal(err)
			}

			// Grab a handle to the mirror.
			mirror := tc.mirror

			// Create a test in-memory server.
			r := mux.NewRouter()

			indexPath := urlJoin(mirror.ExportRoot, "index.txt")
			r.HandleFunc("/"+indexPath, func(w http.ResponseWriter, r *http.Request) {
				for _, v := range tc.indexFiles {
					fmt.Fprintln(w, v)
				}
			})
			for _, indexFile := range tc.indexFiles {
				indexFilePath := urlJoin(mirror.ExportRoot, indexFile)
				r.HandleFunc("/"+indexFilePath, func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/zip")
					fmt.Fprint(w, zipFileContents)
				})
			}

			ts := httptest.NewServer(r)
			t.Cleanup(ts.Close)

			// Set up mirror and save to the database.
			mirror.IndexFile = urlJoin(ts.URL, indexPath)
			mirror.ExportRoot = urlJoin(ts.URL, mirror.ExportRoot)
			if err := mirrorDB.AddMirror(ctx, tc.mirror); err != nil {
				t.Fatal(err)
			}
			if err := mirrorDB.SaveFiles(ctx, mirror.ID, tc.mirrorFiles); err != nil {
				t.Fatal(err)
			}

			// Save any files into the blobstore.
			for _, f := range tc.mirrorFiles {
				name := f.RemoteFile
				if f.LocalFile != "" {
					name = f.LocalFile
				}
				pth := urlJoin(mirror.FilenameRoot, name)
				if err := testBlobstore.CreateObject(ctx, mirror.CloudStorageBucket, pth, nil, true, storage.ContentTypeZip); err != nil {
					t.Fatal(err)
				}
			}

			deadline := time.Now().Add(60 * time.Second)
			if err := s.processMirror(ctx, deadline, mirror); err != nil {
				t.Fatal(err)
			}

			// Verify that expected files were deleted from the blobstore.
			for _, exp := range tc.expBlobstoreDeleted {
				if _, err := testBlobstore.GetObject(ctx, exp[0], exp[1]); !errors.Is(err, storage.ErrNotFound) {
					t.Errorf("%s: expected %v, got %v", urlJoin(exp[0], exp[1]), storage.ErrNotFound, err)
				}
			}

			// Verify that expected files were added to the blobstore.
			for _, exp := range tc.expBlobstoreAdded {
				b, err := testBlobstore.GetObject(ctx, exp[0], exp[1])
				if err != nil {
					t.Errorf("%s: %s", urlJoin(exp[0], exp[1]), err)
				}
				if got, want := string(b), zipFileContents; got != want {
					t.Errorf("expected %q to be %q", got, want)
				}
			}
		})
	}
}

func TestServer_DownloadIndex(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	emptyDB := &database.DB{}
	testBlobstore, err := storage.NewMemory(ctx, &storage.Config{})
	if err != nil {
		t.Fatal(err)
	}

	env := serverenv.New(ctx,
		serverenv.WithDatabase(emptyDB),
		serverenv.WithBlobStorage(testBlobstore),
	)

	// Client honors the provided timeout.
	t.Run("timeout", func(t *testing.T) {
		t.Parallel()

		c := &Config{IndexFileDownloadTimeout: 1 * time.Nanosecond}
		s, err := NewServer(c, env)
		if err != nil {
			t.Fatal(err)
		}

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(250 * time.Millisecond)
			fmt.Fprint(w, "abc-123-456.zip")
		}))
		t.Cleanup(ts.Close)

		_, err = s.downloadIndex(ctx, &mirrormodel.Mirror{
			IndexFile: ts.URL,
		})

		var terr *url.Error
		if errors.As(err, &terr) {
			if !terr.Timeout() {
				t.Errorf("expected %#v to be a timeout error", terr)
			}
		} else {
			t.Fatalf("expected url.Error, got %#v", err)
		}
	})

	// Client returns an error on non-200.
	t.Run("non_200", func(t *testing.T) {
		t.Parallel()

		c := &Config{}
		s, err := NewServer(c, env)
		if err != nil {
			t.Fatal(err)
		}

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(404)
		}))
		t.Cleanup(ts.Close)

		_, err = s.downloadIndex(ctx, &mirrormodel.Mirror{
			IndexFile: ts.URL,
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if got, want := err.Error(), "failed to download"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
		}
	})

	// Client only reads up to configured bytes limit
	t.Run("max_bytes", func(t *testing.T) {
		t.Parallel()

		c := &Config{MaxIndexBytes: 1}
		s, err := NewServer(c, env)
		if err != nil {
			t.Fatal(err)
		}

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "abc-123-456.zip")
			fmt.Fprint(w, "abc-123-456.zip")
			fmt.Fprint(w, "abc-123-456.zip")
		}))
		t.Cleanup(ts.Close)

		_, err = s.downloadIndex(ctx, &mirrormodel.Mirror{
			IndexFile: ts.URL,
		})
		if got, want := err.Error(), "response exceeds 1 bytes"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
		}
	})

	// Client handles empty index file
	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		c := &Config{}
		s, err := NewServer(c, env)
		if err != nil {
			t.Fatal(err)
		}

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		t.Cleanup(ts.Close)

		results, err := s.downloadIndex(ctx, &mirrormodel.Mirror{
			IndexFile: ts.URL,
		})
		if err != nil {
			t.Fatal(err)
		}

		if len(results) != 0 {
			t.Errorf("expected results to be empty, got %#v", results)
		}
	})

	// Client handles multiple files
	t.Run("multifile", func(t *testing.T) {
		t.Parallel()

		c := &Config{
			MaxIndexBytes: 8196,
		}
		s, err := NewServer(c, env)
		if err != nil {
			t.Fatal(err)
		}

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			now := time.Now().UTC().Second()

			for i := 1; i <= 50; i++ {
				for j := 1; j <= 5; j++ {
					fmt.Fprintf(w, "us/%d-%d-%05d.zip\n", now+(i*1000), now+(i*2000), j)
				}
			}
		}))
		t.Cleanup(ts.Close)

		results, err := s.downloadIndex(ctx, &mirrormodel.Mirror{
			IndexFile: ts.URL,
		})
		if err != nil {
			t.Fatal(err)
		}

		if got, want := len(results), 250; got != want {
			t.Errorf("got %d results, expected %d", got, want)
		}
	})

	// Client handles leading and trailing slashes
	t.Run("slashes", func(t *testing.T) {
		t.Parallel()

		c := &Config{
			MaxIndexBytes: 8196,
		}
		s, err := NewServer(c, env)
		if err != nil {
			t.Fatal(err)
		}

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "us/1-2-00001.zip\n")
			fmt.Fprintf(w, "/us/2-3-00008.zip\n")
		}))
		t.Cleanup(ts.Close)

		for _, root := range []string{
			"https://example.com/path",
			"https://example.com/path/",
		} {
			results, err := s.downloadIndex(ctx, &mirrormodel.Mirror{
				IndexFile:  ts.URL,
				ExportRoot: root,
			})
			if err != nil {
				t.Fatal(err)
			}

			if got, want := len(results), 2; got != want {
				t.Errorf("got %d results, expected %d", got, want)
			}
			sort.Strings(results)

			if got, want := results[0], "https://example.com/path/us/1-2-00001.zip"; got != want {
				t.Errorf("expected %q to be %q", got, want)
			}
			if got, want := results[1], "https://example.com/path/us/2-3-00008.zip"; got != want {
				t.Errorf("expected %q to be %q", got, want)
			}
		}
	})
}

func TestServer_ComputeActions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		knownFiles []*mirrormodel.MirrorFile
		indexFiles []string
		exp        map[string]*FileStatus
	}{
		{
			name:       "nil",
			knownFiles: nil,
			indexFiles: nil,
			exp:        map[string]*FileStatus{},
		},
		{
			name:       "empty",
			knownFiles: []*mirrormodel.MirrorFile{},
			indexFiles: []string{},
			exp:        map[string]*FileStatus{},
		},
		{
			name:       "all_index",
			knownFiles: []*mirrormodel.MirrorFile{},
			indexFiles: []string{
				"us/1605818705-1605819005-00001.zip",
				"us/1605818705-1605819005-00002.zip",
			},
			exp: map[string]*FileStatus{
				"1605818705-1605819005-00001.zip": {
					Order:        1,
					DownloadPath: "us/1605818705-1605819005-00001.zip",
					Filename:     "1605818705-1605819005-00001.zip",
				},
				"1605818705-1605819005-00002.zip": {
					Order:        2,
					DownloadPath: "us/1605818705-1605819005-00002.zip",
					Filename:     "1605818705-1605819005-00002.zip",
				},
			},
		},
		{
			name: "all_known",
			knownFiles: []*mirrormodel.MirrorFile{
				{
					MirrorID:      1,
					Filename:      "1605818705-1605819005-00001.zip",
					LocalFilename: stringPtr("1605818705-1605819005-00001.zip"),
				},
				{
					MirrorID:      1,
					Filename:      "1605818705-1605819005-00002.zip",
					LocalFilename: stringPtr("1605818705-1605819005-00002.zip"),
				},
			},
			indexFiles: []string{},
			exp: map[string]*FileStatus{
				"1605818705-1605819005-00001.zip": {
					MirrorFile: &mirrormodel.MirrorFile{
						MirrorID:      1,
						Filename:      "1605818705-1605819005-00001.zip",
						LocalFilename: stringPtr("1605818705-1605819005-00001.zip"),
					},
				},
				"1605818705-1605819005-00002.zip": {
					MirrorFile: &mirrormodel.MirrorFile{
						MirrorID:      1,
						Filename:      "1605818705-1605819005-00002.zip",
						LocalFilename: stringPtr("1605818705-1605819005-00002.zip"),
					},
				},
			},
		},
		{
			name: "intersect",
			knownFiles: []*mirrormodel.MirrorFile{
				{
					MirrorID:      1,
					Filename:      "1605818705-1605819005-00001.zip",
					LocalFilename: stringPtr("1605818705-1605819005-00001.zip"),
				},
				{
					MirrorID:      1,
					Filename:      "1605818705-1605819005-00002.zip",
					LocalFilename: stringPtr("1605818705-1605819005-00002.zip"),
				},
			},
			indexFiles: []string{
				"ca/1605818705-1605819005-00001.zip",
				"us/1605818705-1605819005-00002.zip",
			},
			exp: map[string]*FileStatus{
				"1605818705-1605819005-00001.zip": {
					Order:         1,
					DownloadPath:  "ca/1605818705-1605819005-00001.zip",
					Filename:      "1605818705-1605819005-00001.zip",
					LocalFilename: "1605818705-1605819005-00001.zip",
					MirrorFile: &mirrormodel.MirrorFile{
						MirrorID:      1,
						Filename:      "1605818705-1605819005-00001.zip",
						LocalFilename: stringPtr("1605818705-1605819005-00001.zip"),
					},
				},
				"1605818705-1605819005-00002.zip": {
					Order:         2,
					DownloadPath:  "us/1605818705-1605819005-00002.zip",
					Filename:      "1605818705-1605819005-00002.zip",
					LocalFilename: "1605818705-1605819005-00002.zip",
					MirrorFile: &mirrormodel.MirrorFile{
						MirrorID:      1,
						Filename:      "1605818705-1605819005-00002.zip",
						LocalFilename: stringPtr("1605818705-1605819005-00002.zip"),
					},
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actions := computeActions(tc.knownFiles, tc.indexFiles)
			if diff := cmp.Diff(tc.exp, actions); diff != "" {
				t.Fatalf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
