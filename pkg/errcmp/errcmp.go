// Copyright 2021 Google LLC
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

// Package errcmp contins a convince helper for checking error conditions in tests
package errcmp

import (
	"strings"
	"testing"
)

func MustMatch(t testing.TB, err error, want string) {
	t.Helper()

	if err == nil {
		if want != "" {
			t.Fatalf("missing error, want: %q got: nil", want)
		}
		// want == "" is success
	} else if err != nil {
		if want == "" {
			t.Fatalf("unexpected error: got: %v", err)
		} else if !strings.Contains(err.Error(), want) {
			t.Fatalf("wrong error; want: %q got: %v", want, err)
		}
	}
}
