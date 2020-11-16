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

package model

import (
	"strings"
	"testing"
)

func TestRewriteFilenameNoOp(t *testing.T) {
	m := Mirror{}

	want := "afile.zip"
	if got, err := m.RewriteFilename(want); err != nil {
		t.Fatalf("unexpected error: %v", err)
	} else if want != got {
		t.Fatalf("filename should not have been rewritten: want: %q got: %q", want, got)
	}
}

func TestRewriteFilenameTimestamps(t *testing.T) {
	pattern := "[timestamp]-[timestamp]-00001.zip"
	m := Mirror{
		FilenameRewrite: &pattern,
	}

	got, err := m.RewriteFilename("random...")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parts := strings.Split(got, "-")
	if len(parts) != 3 {
		t.Fatalf("filename doesn't have 3 parts: got: %v", parts)
	}

	if parts[0] == parts[1] {
		t.Fatalf("got 2 identical timestamps...")
	}
}

func TestRewriteUUID(t *testing.T) {
	pattern := "[uuid]-00001.zip"
	m := Mirror{
		FilenameRewrite: &pattern,
	}

	got, err := m.RewriteFilename("random...")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parts := strings.Split(got, "-")
	if len(parts) != 6 {
		t.Fatalf("didn't get a uuid: %q", got)
	}
}
