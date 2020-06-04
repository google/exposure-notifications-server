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

package envconfig

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

var _ Decoder = (*CustomType)(nil)

// CustomType is used to test custom decode methods.
type CustomType struct {
	value string
}

func (c *CustomType) EnvDecode(val string) error {
	c.value = "CUSTOM-" + val
	return nil
}

var _ Decoder = (*CustomTypeError)(nil)

// CustomTypeError returns an error on the custom decoder.
type CustomTypeError struct {
	Field string
}

func (c *CustomTypeError) EnvDecode(val string) error {
	return fmt.Errorf("broken")
}

// Electron > Lepton > Quark
type Electron struct {
	Name   string `env:"ELECTRON_NAME"`
	Lepton *Lepton
}

type Lepton struct {
	Name  string `env:"LEPTON_NAME"`
	Quark Quark
}

type Quark struct {
	Value int8 `env:"QUARK_VALUE"`
}

func TestProcessWith(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    interface{}
		exp      interface{}
		lookuper Lookuper
		err      error
		errMsg   string
	}{
		// nil pointer
		{
			name:     "nil",
			input:    (*Electron)(nil),
			lookuper: MapLookuper(map[string]string{}),
			err:      ErrNotStruct,
		},

		// Bool
		{
			name: "bool/true",
			input: &struct {
				Field bool `env:"FIELD"`
			}{},
			exp: &struct {
				Field bool `env:"FIELD"`
			}{
				Field: true,
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "true",
			}),
		},
		{
			name: "bool/false",
			input: &struct {
				Field bool `env:"FIELD"`
			}{},
			exp: &struct {
				Field bool `env:"FIELD"`
			}{
				Field: false,
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "false",
			}),
		},
		{
			name: "bool/error",
			input: &struct {
				Field bool `env:"FIELD"`
			}{},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "not a valid bool",
			}),
			errMsg: "invalid syntax",
		},

		// Float
		{
			name: "float32/6.022",
			input: &struct {
				Field float32 `env:"FIELD"`
			}{},
			exp: &struct {
				Field float32 `env:"FIELD"`
			}{
				Field: 6.022,
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "6.022",
			}),
		},
		{
			name: "float32/error",
			input: &struct {
				Field float32 `env:"FIELD"`
			}{},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "not a valid float",
			}),
			errMsg: "invalid syntax",
		},
		{
			name: "float64/6.022",
			input: &struct {
				Field float64 `env:"FIELD"`
			}{},
			exp: &struct {
				Field float64 `env:"FIELD"`
			}{
				Field: 6.022,
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "6.022",
			}),
		},
		{
			name: "float32/error",
			input: &struct {
				Field float64 `env:"FIELD"`
			}{},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "not a valid float",
			}),
			errMsg: "invalid syntax",
		},

		// Int8-32
		{
			name: "int/8675309",
			input: &struct {
				Field int `env:"FIELD"`
			}{},
			exp: &struct {
				Field int `env:"FIELD"`
			}{
				Field: 8675309,
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "8675309",
			}),
		},
		{
			name: "int/error",
			input: &struct {
				Field int `env:"FIELD"`
			}{},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "not a valid int",
			}),
			errMsg: "invalid syntax",
		},
		{
			name: "int8/12",
			input: &struct {
				Field int8 `env:"FIELD"`
			}{},
			exp: &struct {
				Field int8 `env:"FIELD"`
			}{
				Field: 12,
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "12",
			}),
		},
		{
			name: "int8/error",
			input: &struct {
				Field int8 `env:"FIELD"`
			}{},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "not a valid int",
			}),
			errMsg: "invalid syntax",
		},
		{
			name: "int16/1245",
			input: &struct {
				Field int16 `env:"FIELD"`
			}{},
			exp: &struct {
				Field int16 `env:"FIELD"`
			}{
				Field: 12345,
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "12345",
			}),
		},
		{
			name: "int16/error",
			input: &struct {
				Field int16 `env:"FIELD"`
			}{},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "not a valid int",
			}),
			errMsg: "invalid syntax",
		},
		{
			name: "int32/1245",
			input: &struct {
				Field int32 `env:"FIELD"`
			}{},
			exp: &struct {
				Field int32 `env:"FIELD"`
			}{
				Field: 12345,
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "12345",
			}),
		},
		{
			name: "int32/error",
			input: &struct {
				Field int32 `env:"FIELD"`
			}{},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "not a valid int",
			}),
			errMsg: "invalid syntax",
		},

		// Int64
		{
			name: "int64/1245",
			input: &struct {
				Field int64 `env:"FIELD"`
			}{},
			exp: &struct {
				Field int64 `env:"FIELD"`
			}{
				Field: 12345,
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "12345",
			}),
		},
		{
			name: "int64/error",
			input: &struct {
				Field int64 `env:"FIELD"`
			}{},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "not a valid int",
			}),
			errMsg: "invalid syntax",
		},
		{
			name: "int64/duration",
			input: &struct {
				Field time.Duration `env:"FIELD"`
			}{},
			exp: &struct {
				Field time.Duration `env:"FIELD"`
			}{
				Field: 10 * time.Second,
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "10s",
			}),
		},
		{
			name: "int64/duration_pointer",
			input: &struct {
				Field *time.Duration `env:"FIELD"`
			}{},
			exp: &struct {
				Field *time.Duration `env:"FIELD"`
			}{
				Field: func() *time.Duration { d := 10 * time.Second; return &d }(),
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "10s",
			}),
		},
		{
			name: "int64/duration_error",
			input: &struct {
				Field time.Duration `env:"FIELD"`
			}{},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "not a valid time",
			}),
			errMsg: "invalid duration",
		},

		// String
		{
			name: "string",
			input: &struct {
				Field string `env:"FIELD"`
			}{},
			exp: &struct {
				Field string `env:"FIELD"`
			}{
				Field: "foo",
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "foo",
			}),
		},

		// Uint8-64
		{
			name: "uint/8675309",
			input: &struct {
				Field uint `env:"FIELD"`
			}{},
			exp: &struct {
				Field uint `env:"FIELD"`
			}{
				Field: 8675309,
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "8675309",
			}),
		},
		{
			name: "uint/error",
			input: &struct {
				Field uint `env:"FIELD"`
			}{},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "not a valid uint",
			}),
			errMsg: "invalid syntax",
		},
		{
			name: "uint8/12",
			input: &struct {
				Field uint8 `env:"FIELD"`
			}{},
			exp: &struct {
				Field uint8 `env:"FIELD"`
			}{
				Field: 12,
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "12",
			}),
		},
		{
			name: "uint8/error",
			input: &struct {
				Field uint8 `env:"FIELD"`
			}{},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "not a valid uint",
			}),
			errMsg: "invalid syntax",
		},
		{
			name: "uint16/1245",
			input: &struct {
				Field uint16 `env:"FIELD"`
			}{},
			exp: &struct {
				Field uint16 `env:"FIELD"`
			}{
				Field: 12345,
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "12345",
			}),
		},
		{
			name: "uint16/error",
			input: &struct {
				Field uint16 `env:"FIELD"`
			}{},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "not a valid uint",
			}),
			errMsg: "invalid syntax",
		},
		{
			name: "uint32/1245",
			input: &struct {
				Field uint32 `env:"FIELD"`
			}{},
			exp: &struct {
				Field uint32 `env:"FIELD"`
			}{
				Field: 12345,
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "12345",
			}),
		},
		{
			name: "uint32/error",
			input: &struct {
				Field uint32 `env:"FIELD"`
			}{},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "not a valid int",
			}),
			errMsg: "invalid syntax",
		},
		{
			name: "uint64/1245",
			input: &struct {
				Field uint64 `env:"FIELD"`
			}{},
			exp: &struct {
				Field uint64 `env:"FIELD"`
			}{
				Field: 12345,
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "12345",
			}),
		},
		{
			name: "uint64/error",
			input: &struct {
				Field uint64 `env:"FIELD"`
			}{},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "not a valid int",
			}),
			errMsg: "invalid syntax",
		},
		{
			name: "uintptr/1245",
			input: &struct {
				Field uintptr `env:"FIELD"`
			}{},
			exp: &struct {
				Field uintptr `env:"FIELD"`
			}{
				Field: 12345,
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "12345",
			}),
		},
		{
			name: "uintptr/error",
			input: &struct {
				Field uintptr `env:"FIELD"`
			}{},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "not a valid int",
			}),
			errMsg: "invalid syntax",
		},

		// Map
		{
			name: "map/single",
			input: &struct {
				Field map[string]string `env:"FIELD"`
			}{},
			exp: &struct {
				Field map[string]string `env:"FIELD"`
			}{
				Field: map[string]string{"foo": "bar"},
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "foo:bar",
			}),
		},
		{
			name: "map/multi",
			input: &struct {
				Field map[string]string `env:"FIELD"`
			}{},
			exp: &struct {
				Field map[string]string `env:"FIELD"`
			}{
				Field: map[string]string{
					"foo":  "bar",
					"zip":  "zap",
					"zing": "zang",
				},
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "foo:bar,zip:zap,zing:zang",
			}),
		},
		{
			name: "map/empty",
			input: &struct {
				Field map[string]string `env:"FIELD"`
			}{},
			exp: &struct {
				Field map[string]string `env:"FIELD"`
			}{
				Field: nil,
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "",
			}),
		},
		{
			name: "map/key_no_value",
			input: &struct {
				Field map[string]string `env:"FIELD"`
			}{},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "foo",
			}),
			errMsg: "invalid map item",
		},
		{
			name: "map/key_error",
			input: &struct {
				Field map[bool]bool `env:"FIELD"`
			}{},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "nope:true",
			}),
			errMsg: "invalid syntax",
		},
		{
			name: "map/value_error",
			input: &struct {
				Field map[bool]bool `env:"FIELD"`
			}{},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "true:nope",
			}),
			errMsg: "invalid syntax",
		},

		// Slices
		{
			name: "slice/single",
			input: &struct {
				Field []string `env:"FIELD"`
			}{},
			exp: &struct {
				Field []string `env:"FIELD"`
			}{
				Field: []string{"foo"},
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "foo",
			}),
		},
		{
			name: "slice/multi",
			input: &struct {
				Field []string `env:"FIELD"`
			}{},
			exp: &struct {
				Field []string `env:"FIELD"`
			}{
				Field: []string{"foo", "bar"},
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "foo,bar",
			}),
		},
		{
			name: "slice/empty",
			input: &struct {
				Field []string `env:"FIELD"`
			}{},
			exp: &struct {
				Field []string `env:"FIELD"`
			}{
				Field: nil,
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "",
			}),
		},
		{
			name: "slice/bytes",
			input: &struct {
				Field []byte `env:"FIELD"`
			}{},
			exp: &struct {
				Field []byte `env:"FIELD"`
			}{
				Field: []byte("foo"),
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "foo",
			}),
		},

		// Private fields
		{
			name: "private/noop",
			input: &struct {
				field string
			}{},
			exp: &struct {
				field string
			}{
				field: "",
			},
			lookuper: MapLookuper(map[string]string{}),
		},
		{
			name: "private/error",
			input: &struct {
				field string `env:"FIELD"`
			}{},
			exp: &struct {
				field string `env:"FIELD"`
			}{},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "foo",
			}),
			err: ErrPrivateField,
		},

		// Required
		{
			name: "required/present",
			input: &struct {
				Field string `env:"FIELD,required"`
			}{},
			exp: &struct {
				Field string `env:"FIELD,required"`
			}{
				Field: "foo",
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "foo",
			}),
		},
		{
			name: "required/missing",
			input: &struct {
				Field string `env:"FIELD,required"`
			}{},
			lookuper: MapLookuper(map[string]string{}),
			err:      ErrMissingRequired,
		},
		{
			name: "required/default",
			input: &struct {
				Field string `env:"FIELD,required,default=foo"`
			}{},
			lookuper: MapLookuper(map[string]string{}),
			err:      ErrRequiredAndDefault,
		},

		// Default
		{
			name: "default/missing",
			input: &struct {
				Field string `env:"FIELD,default=foo"`
			}{},
			exp: &struct {
				Field string `env:"FIELD,default=foo"`
			}{
				Field: "foo", // uses default
			},
			lookuper: MapLookuper(map[string]string{}),
		},
		{
			name: "default/empty",
			input: &struct {
				Field string `env:"FIELD,default=foo"`
			}{},
			exp: &struct {
				Field string `env:"FIELD,default=foo"`
			}{
				Field: "", // doesn't use default
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "",
			}),
		},
		{
			name: "default/expand",
			input: &struct {
				Field string `env:"FIELD,default=$DEFAULT"`
			}{},
			exp: &struct {
				Field string `env:"FIELD,default=$DEFAULT"`
			}{
				Field: "bar",
			},
			lookuper: MapLookuper(map[string]string{
				"DEFAULT": "bar",
			}),
		},

		// Custom decoder
		{
			name: "custom_decoder/struct",
			input: &struct {
				Field CustomType `env:"FIELD"`
			}{},
			exp: &struct {
				Field CustomType `env:"FIELD"`
			}{
				Field: CustomType{
					value: "CUSTOM-foo",
				},
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "foo",
			}),
		},
		{
			name: "custom_decoder/pointer",
			input: &struct {
				Field *CustomType `env:"FIELD"`
			}{},
			exp: &struct {
				Field *CustomType `env:"FIELD"`
			}{
				Field: &CustomType{
					value: "CUSTOM-foo",
				},
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "foo",
			}),
		},
		{
			name: "custom_decoder/private",
			input: &struct {
				field *CustomType `env:"FIELD"`
			}{},
			lookuper: MapLookuper(map[string]string{}),
			err:      ErrPrivateField,
		},
		{
			name: "custom_decoder/error",
			input: &struct {
				Field CustomTypeError `env:"FIELD"`
			}{},
			lookuper: MapLookuper(map[string]string{}),
			errMsg:   "broken",
		},

		// Pointer pointers
		{
			name: "string_pointer",
			input: &struct {
				Field *string `env:"FIELD"`
			}{},
			exp: &struct {
				Field *string `env:"FIELD"`
			}{
				Field: func() *string { s := "foo"; return &s }(),
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "foo",
			}),
		},
		{
			name: "string_pointer_pointer",
			input: &struct {
				Field **string `env:"FIELD"`
			}{},
			exp: &struct {
				Field **string `env:"FIELD"`
			}{
				Field: func() **string { s := "foo"; ptr := &s; return &ptr }(),
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "foo",
			}),
		},
		{
			name: "map_pointer",
			input: &struct {
				Field *map[string]string `env:"FIELD"`
			}{},
			exp: &struct {
				Field *map[string]string `env:"FIELD"`
			}{
				Field: func() *map[string]string {
					m := map[string]string{"foo": "bar"}
					return &m
				}(),
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "foo:bar",
			}),
		},
		{
			name: "slice_pointer",
			input: &struct {
				Field *[]string `env:"FIELD"`
			}{},
			exp: &struct {
				Field *[]string `env:"FIELD"`
			}{
				Field: func() *[]string {
					s := []string{"foo"}
					return &s
				}(),
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "foo",
			}),
		},

		// Nesting
		{
			name:  "nested_pointer_structs",
			input: &Electron{},
			exp: &Electron{
				Name: "shocking",
				Lepton: &Lepton{
					Name: "tea?",
					Quark: Quark{
						Value: 2,
					},
				},
			},
			lookuper: MapLookuper(map[string]string{
				"ELECTRON_NAME": "shocking",
				"LEPTON_NAME":   "tea?",
				"QUARK_VALUE":   "2",
			}),
		},

		// Overwriting
		{
			name: "no_overwrite/structs",
			input: &Electron{
				Name: "original",
				Lepton: &Lepton{
					Name: "original",
					Quark: Quark{
						Value: 1,
					},
				},
			},
			exp: &Electron{
				Name: "original",
				Lepton: &Lepton{
					Name: "original",
					Quark: Quark{
						Value: 1,
					},
				},
			},
			lookuper: MapLookuper(map[string]string{
				"ELECTRON_NAME": "shocking",
				"LEPTON_NAME":   "tea?",
				"QUARK_VALUE":   "2",
			}),
		},
		{
			name: "no_overwrite/pointers",
			input: &struct {
				Field *string `env:"FIELD"`
			}{
				Field: func() *string { s := "bar"; return &s }(),
			},
			exp: &struct {
				Field *string `env:"FIELD"`
			}{
				Field: func() *string { s := "bar"; return &s }(),
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "foo",
			}),
		},
		{
			name: "no_overwrite/pointers_pointers",
			input: &struct {
				Field **string `env:"FIELD"`
			}{
				Field: func() **string {
					s := "bar"
					ptr := &s
					return &ptr
				}(),
			},
			exp: &struct {
				Field **string `env:"FIELD"`
			}{
				Field: func() **string {
					s := "bar"
					ptr := &s
					return &ptr
				}(),
			},
			lookuper: MapLookuper(map[string]string{
				"FIELD": "foo",
			}),
		},

		// Unknown options
		{
			name: "unknown_options",
			input: &struct {
				Field string `env:"FIELD,cookies"`
			}{},
			lookuper: MapLookuper(map[string]string{}),
			err:      ErrUnknownOption,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := ProcessWith(tc.input, tc.lookuper); err != nil {
				if tc.err == nil && tc.errMsg == "" {
					t.Fatal(err)
				}

				if tc.err != nil && !errors.Is(err, tc.err) {
					t.Fatalf("expected \n%+v\n to be \n%+v\n", err, tc.err)
				}

				if got, want := err.Error(), tc.errMsg; want != "" && !strings.Contains(got, want) {
					t.Fatalf("expected \n%+v\n to match \n%+v\n", got, want)
				}

				// There's an error, but it passed all our tests, so return now.
				return
			}

			opts := cmp.AllowUnexported(
				// Custom decoder type
				CustomType{},

				// Custom decoder type that returns an error
				CustomTypeError{},

				// Anonymous struct with private fields
				struct{ field string }{},
			)
			if diff := cmp.Diff(tc.exp, tc.input, opts); diff != "" {
				t.Fatalf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
