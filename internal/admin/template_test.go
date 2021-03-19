// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, softwar
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package admin

import (
	"testing"
	"time"
)

func TestTimestampFormatter(t *testing.T) {
	t.Parallel()

	formatter := timestampFormatter("2006-01-02")

	cases := []struct {
		name string
		in   interface{}
		exp  string
		err  bool
	}{
		{
			name: "nil",
			in:   nil,
			exp:  "",
		},
		{
			name: "time_zero",
			in:   time.Time{},
			exp:  "",
		},
		{
			name: "time_non_zero",
			in:   time.Unix(1000000000, 0).UTC(),
			exp:  "2001-09-09",
		},
		{
			name: "time_ptr_nil",
			in:   (*time.Time)(nil),
			exp:  "",
		},
		{
			name: "time_ptr_zero",
			in:   &time.Time{},
			exp:  "",
		},
		{
			name: "time_ptr_non_zero",
			in:   timePtr(time.Unix(1000000000, 0).UTC()),
			exp:  "2001-09-09",
		},
		{
			name: "string",
			in:   "foo",
			exp:  "foo",
		},
		{
			name: "other",
			in:   64,
			err:  true,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := formatter(tc.in)
			if (err != nil) != tc.err {
				t.Fatal(err)
			}

			if got, want := got, tc.exp; got != want {
				t.Errorf("expected %q to be %q", got, want)
			}
		})
	}
}

func TestDeref(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   interface{}
		exp  string
		err  bool
	}{
		{
			name: "nil",
			in:   nil,
			exp:  "",
		},
		{
			name: "unknown",
			in:   &struct{}{},
			err:  true,
		},
		{
			name: "string_ptr",
			in:   stringPtr("hi"),
			exp:  "hi",
		},
		{
			name: "string_ptr_nil",
			in:   (*string)(nil),
			exp:  "",
		},
		{
			name: "int_ptr",
			in:   intPtr(123),
			exp:  "123",
		},
		{
			name: "int_ptr_nil",
			in:   (*int)(nil),
			exp:  "0",
		},
		{
			name: "int8_ptr",
			in:   int8Ptr(123),
			exp:  "123",
		},
		{
			name: "int8_ptr_nil",
			in:   (*int8)(nil),
			exp:  "0",
		},
		{
			name: "int16_ptr",
			in:   int16Ptr(123),
			exp:  "123",
		},
		{
			name: "int16_ptr_nil",
			in:   (*int16)(nil),
			exp:  "0",
		},
		{
			name: "int32_ptr",
			in:   int32Ptr(123),
			exp:  "123",
		},
		{
			name: "int32_ptr_nil",
			in:   (*int32)(nil),
			exp:  "0",
		},
		{
			name: "int64_ptr",
			in:   int64Ptr(123),
			exp:  "123",
		},
		{
			name: "int64_ptr_nil",
			in:   (*int64)(nil),
			exp:  "0",
		},
		{
			name: "uint_ptr",
			in:   uintPtr(123),
			exp:  "123",
		},
		{
			name: "uint_ptr_nil",
			in:   (*uint)(nil),
			exp:  "0",
		},
		{
			name: "uint8_ptr",
			in:   uint8Ptr(123),
			exp:  "123",
		},
		{
			name: "uint8_ptr_nil",
			in:   (*uint8)(nil),
			exp:  "0",
		},
		{
			name: "uint16_ptr",
			in:   uint16Ptr(123),
			exp:  "123",
		},
		{
			name: "uint16_ptr_nil",
			in:   (*uint16)(nil),
			exp:  "0",
		},
		{
			name: "uint32_ptr",
			in:   uint32Ptr(123),
			exp:  "123",
		},
		{
			name: "uint32_ptr_nil",
			in:   (*uint32)(nil),
			exp:  "0",
		},
		{
			name: "uint64_ptr",
			in:   uint64Ptr(123),
			exp:  "123",
		},
		{
			name: "uint64_ptr_nil",
			in:   (*uint64)(nil),
			exp:  "0",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := deref(tc.in)
			if (err != nil) != tc.err {
				t.Fatal(err)
			}

			if got, want := got, tc.exp; got != want {
				t.Errorf("expected %q to be %q", got, want)
			}
		})
	}
}
