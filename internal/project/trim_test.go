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

package project

import (
	"testing"
)

func TestTrimSpaceAndNonPrintable_unicode(t *testing.T) {
	t.Parallel()

	extraChars := "state\uFEFF"
	want := "state"
	got := TrimSpaceAndNonPrintable(extraChars)

	if want != got {
		t.Fatalf("wrong trim, want: %q got: %q", want, got)
	}
}

func TestTrimSpaceAndNonPrintable_space(t *testing.T) {
	t.Parallel()

	extraChars := " state  \r\t"
	want := "state"
	got := TrimSpaceAndNonPrintable(extraChars)

	if want != got {
		t.Fatalf("wrong trim, want: %q got: %q", want, got)
	}
}

func TestTrimSpace_unicode(t *testing.T) {
	t.Parallel()

	extraChars := "state\uFEFF"
	want := "state"
	got := TrimSpace(extraChars)

	if want != got {
		t.Fatalf("wrong trim, want: %q got: %q", want, got)
	}
}

func TestTrimSpace_space(t *testing.T) {
	t.Parallel()

	extraChars := " state  \r\t"
	want := "state"
	got := TrimSpace(extraChars)

	if want != got {
		t.Fatalf("wrong trim, want: %q got: %q", want, got)
	}
}
