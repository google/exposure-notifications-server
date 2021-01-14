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

package base64util

import (
	"testing"
)

func TestDecodeString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		encoded string
		raw     string
	}{
		{"FPucA9l+", "\x14\xfb\x9c\x03\xd9\x7e"},
		{"FPucA9l-", "\x14\xfb\x9c\x03\xd9\x7e"},
		{"FPucA9k=", "\x14\xfb\x9c\x03\xd9"},
		{"FPucA9k", "\x14\xfb\x9c\x03\xd9"},
		{"FPucAw==", "\x14\xfb\x9c\x03"},
		{"FPucAw", "\x14\xfb\x9c\x03"},
		{"", ""},
		{"=", ""},
		{"==", ""},
		{"Zg==", "f"},
		{"Zg=", "f"},
		{"Zg", "f"},
	}

	for _, c := range cases {
		c := c

		decoded, err := DecodeString(c.encoded)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := string(decoded), c.raw; got != want {
			t.Errorf("expected %q to be %q", got, want)
		}
	}
}
