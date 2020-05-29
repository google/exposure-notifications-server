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
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"testing"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

func testGoogleCloudStorageBucket(tb testing.TB) string {
	tb.Helper()

	if testing.Short() {
		tb.Skipf("🚧 Skipping Google Cloud Storage tests (short!")
	}

	if skip, _ := strconv.ParseBool(os.Getenv("SKIP_GOOGLE_CLOUD_STORAGE_TESTS")); skip {
		tb.Skipf("🚧 Skipping Google Cloud Storage tests (SKIP_GOOGLE_CLOUD_STORAGE_TESTS is set)!")
	}

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		tb.Fatal("missing GOOGLE_CLOUD_PROJECT!")
	}

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		tb.Fatal(err)
	}

	// Create a random bucket name.
	var b [512]byte
	if _, err := rand.Read(b[:]); err != nil {
		tb.Fatalf("failed to generate random: %v", err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(b[:]))
	name := fmt.Sprintf("%s-%s", projectID, digest[:16])

	// Create the bucket.
	if err := client.Bucket(name).Create(ctx, projectID, nil); err != nil {
		tb.Fatalf("failed to create bucket: %v", err)
	}

	// Schedule cleanup.
	tb.Cleanup(func() {
		var objs []string
		it := client.Bucket(name).Objects(ctx, &storage.Query{Prefix: ""})
		for {
			attrs, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				tb.Fatal(err)
			}
			objs = append(objs, attrs.Name)
		}

		for _, obj := range objs {
			if err := client.Bucket(name).Object(obj).Delete(ctx); err != nil {
				tb.Errorf("failed to cleanup object %v: %v", obj, err)
			}
		}

		if err := client.Bucket(name).Delete(ctx); err != nil {
			tb.Fatalf("failed to delete bucket: %v", err)
		}
	})

	return name
}

func TestGoogleCloudStorage_CreateObject(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		t.Fatal(err)
	}

	bucket := testGoogleCloudStorageBucket(t)

	cases := []struct {
		name     string
		bucket   string
		filepath string
		contents []byte
		err      bool
	}{
		{
			name:     "default",
			bucket:   bucket,
			filepath: "myfile",
			contents: []byte("contents"),
		},
		{
			name:     "bad_bucket",
			bucket:   "totally-like-not-a-real-bucket",
			filepath: "myfile",
			contents: []byte("contents"),
			err:      true,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gcsStorage, err := NewGoogleCloudStorage(ctx)
			if err != nil {
				t.Fatal(err)
			}

			err = gcsStorage.CreateObject(ctx, tc.bucket, tc.filepath, tc.contents)
			if (err != nil) != tc.err {
				t.Fatal(err)
			}

			if !tc.err {
				r, err := client.Bucket(tc.bucket).Object(tc.filepath).NewReader(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer r.Close()

				contents, err := ioutil.ReadAll(r)
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

func TestGoogleCloudStorage_DeleteObject(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		t.Fatal(err)
	}

	bucket := testGoogleCloudStorageBucket(t)

	file := "my-file.txt"
	w := client.Bucket(bucket).Object(file).NewWriter(ctx)
	if _, err := fmt.Fprintf(w, "hello"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		bucket   string
		filepath string
	}{
		{
			name:     "default",
			bucket:   bucket,
			filepath: file,
		},
		{
			name:     "bucket_not_exist",
			bucket:   "totally-like-not-a-real-bucket",
			filepath: file,
		},
		{
			name:     "file_not_exist",
			bucket:   bucket,
			filepath: "not-exist",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			storage, err := NewFilesystemStorage(ctx)
			if err != nil {
				t.Fatal(err)
			}

			if err = storage.DeleteObject(ctx, tc.bucket, tc.filepath); err != nil {
				t.Fatal(err)
			}
		})
	}
}
