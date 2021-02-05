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
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/google/exposure-notifications-server/internal/project"
)

func maybeSkipCloudStorage(tb testing.TB) {
	tb.Helper()

	if testing.Short() {
		tb.Skipf("🚧 Skipping Google Cloud Storage tests (short)!")
	}

	if skip, _ := strconv.ParseBool(os.Getenv("SKIP_GOOGLE_CLOUD_STORAGE_TESTS")); skip {
		tb.Skipf("🚧 Skipping Google Cloud Storage tests (SKIP_GOOGLE_CLOUD_STORAGE_TESTS is set)!")
	}
}

func testGoogleCloudStorageClient(tb testing.TB) *storage.Client {
	tb.Helper()

	maybeSkipCloudStorage(tb)

	ctx := project.TestContext(tb)
	client, err := storage.NewClient(ctx)
	if err != nil {
		tb.Fatal(err)
	}
	return client
}

func testName(tb testing.TB) string {
	tb.Helper()

	var b [512]byte
	if _, err := rand.Read(b[:]); err != nil {
		tb.Fatalf("failed to generate random: %v", err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(b[:]))
	return digest[:32]
}

func testGoogleCloudStorageBucket(tb testing.TB) string {
	tb.Helper()

	maybeSkipCloudStorage(tb)

	bucketID := os.Getenv("GOOGLE_CLOUD_BUCKET")
	if bucketID == "" {
		tb.Fatal("missing GOOGLE_CLOUD_BUCKET!")
	}

	return bucketID
}

func testGoogleCloudStorageObject(tb testing.TB, r io.Reader) string {
	tb.Helper()

	maybeSkipCloudStorage(tb)

	ctx := project.TestContext(tb)
	client := testGoogleCloudStorageClient(tb)
	bucket := testGoogleCloudStorageBucket(tb)
	name := testName(tb)

	// Create the object.
	w := client.Bucket(bucket).Object(name).NewWriter(ctx)
	if _, err := io.Copy(w, r); err != nil {
		tb.Fatalf("failed to create object: %v", err)
	}
	if err := w.Close(); err != nil {
		tb.Fatalf("failed to close writer: %v", err)
	}

	// Schedule cleanup.
	tb.Cleanup(func() {
		if err := client.Bucket(bucket).Object(name).Delete(ctx); err != nil && err != storage.ErrObjectNotExist {
			tb.Fatalf("failed cleaning up %s: %v", name, err)
		}
	})

	return name
}

func TestGoogleCloudStorage_CreateObject(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	client := testGoogleCloudStorageClient(t)
	bucket := testGoogleCloudStorageBucket(t)
	object := testGoogleCloudStorageObject(t, strings.NewReader("contents"))

	cases := []struct {
		name     string
		bucket   string
		object   string
		contents []byte
		err      bool
	}{
		{
			name:     "default",
			bucket:   bucket,
			object:   testName(t),
			contents: []byte("contents"),
		},
		{
			name:     "already_exists",
			bucket:   bucket,
			object:   object,
			contents: []byte("contents"),
			err:      false,
		},
		{
			name:   "bad_bucket",
			bucket: "totally-like-not-a-real-bucket",
			object: testName(t),
			err:    true,
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

			err = gcsStorage.CreateObject(ctx, tc.bucket, tc.object, tc.contents, false, ContentTypeZip)
			if (err != nil) != tc.err {
				t.Fatal(err)
			}

			if !tc.err {
				r, err := client.Bucket(tc.bucket).Object(tc.object).NewReader(ctx)
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

	ctx := project.TestContext(t)
	client := testGoogleCloudStorageClient(t)
	bucket := testGoogleCloudStorageBucket(t)
	object := testGoogleCloudStorageObject(t, strings.NewReader("contents"))

	cases := []struct {
		name   string
		bucket string
		object string
	}{
		{
			name:   "default",
			bucket: bucket,
			object: object,
		},
		{
			name:   "bucket_not_exist",
			bucket: "totally-like-not-a-real-bucket",
			object: object,
		},
		{
			name:   "file_not_exist",
			bucket: bucket,
			object: "not-exist",
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

			if err := gcsStorage.DeleteObject(ctx, tc.bucket, tc.object); err != nil {
				t.Fatal(err)
			}

			if _, err := client.Bucket(tc.bucket).Object(tc.object).Attrs(ctx); err != storage.ErrObjectNotExist {
				t.Errorf("expected object %v to be deleted", tc.object)
			}
		})
	}
}
