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

// Package envconfig populates struct fields based on environment variable
// values (or anything that responds to "Lookup"). Structs declare their
// environment dependencies using the `env` tag with the key being the name of
// the environment variable, case sensitive.
//
//     type MyStruct struct {
//         A string `env:"A"` // resolves A to $A
//         B string `env:"B,required"` // resolves B to $B, errors if $B is unset
//         C string `env:"C,default=foo"` // resolves C to $C, defaults to "foo"
//
//         D string `env:"D,required,default=foo"` // error, cannot be required and default
//         E string `env:""` // error, must specify key
//     }
//
// All built-in types are supported except Func and Chan. If you need to define
// a custom decoder, implement Decoder:
//
//     type MyStruct struct {
//         field string
//     }
//
//     func (v *MyStruct) EnvDecode(val string) error {
//         v.field = fmt.Sprintf("PREFIX-%s", val)
//         return nil
//     }
//
// In the environment, slices are specified as comma-separated values:
//
//     export MYVAR="a,b,c,d" // []string{"a", "b", "c", "d"}
//
// In the environment, maps are specified as comma-separated key:value pairs:
//
//     export MYVAR="a:b,c:d" // map[string]string{"a":"b", "c":"d"}
//
// If you need to modify environment variable values before processing, you can
// specify a custom mutator:
//
//     type Config struct {
//         Password `env:"PASSWORD_SECRET"`
//     }
//
//     func resolveSecretFunc(key, value string) (string, error) {
//         if strings.HasSuffix(key, "_SECRET") {
//             return secretmanager.Resolve(value)
//         }
//     }
//
//     var config Config
//     ProcessWith(&config, new(OsLookuper), resolveSecretFunc)
//
// It is conceptually similar to kelseyhightower/envconfig, with the following
// major behavioral changes:
//
// - Support for specifying a custom lookup function such as a map, which is
// useful for testing.
//
// - Only populates fields if they contain the zero (or nil) value. This means
// you can pre-initialize a struct and any pre-populated fields will not be
// overwritten during processing.
//
// - Support for interpolation. The default value for a field can be the value
// of another field.
//
// - Support for arbitrary mutators that change/resolve data before type
// conversion.
//
package envconfig

import (
	"context"
	"encoding"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const envTag = "env"

var (
	ErrInvalidMapItem     = fmt.Errorf("invalid map item")
	ErrLookuperNil        = fmt.Errorf("lookuper cannot be nil")
	ErrMissingKey         = fmt.Errorf("missing key")
	ErrMissingRequired    = fmt.Errorf("missing required value")
	ErrNotPtr             = fmt.Errorf("input must be a pointer")
	ErrNotStruct          = fmt.Errorf("input must be a struct")
	ErrPrivateField       = fmt.Errorf("cannot parse private fields")
	ErrRequiredAndDefault = fmt.Errorf("field cannot be required and have a default value")
	ErrUnknownOption      = fmt.Errorf("unknown option")
)

// Lookuper is an interface that provides a lookup for a string-based key.
type Lookuper interface {
	// Lookup searches for the given key and returns the corresponding string
	// value. If a value is found, it returns the value and true. If a value is
	// not found, it returns the empty string and false.
	Lookup(key string) (string, bool)
}

// osLookuper looks up environment configuration from the local environment.
type osLookuper struct{}

// Verify implements interface.
var _ Lookuper = (*osLookuper)(nil)

func (o *osLookuper) Lookup(key string) (string, bool) {
	return os.LookupEnv(key)
}

// OsLookuper returns a lookuper that uses the environment (os.LookupEnv) to
// resolve values.
func OsLookuper() Lookuper {
	return new(osLookuper)
}

type mapLookuper map[string]string

var _ Lookuper = (*mapLookuper)(nil)

func (m mapLookuper) Lookup(key string) (string, bool) {
	v, ok := m[key]
	return v, ok
}

// MapLookuper looks up environment configuration from a provided map. This is
// useful for testing, especially in parallel, since it does not require you to
// mutate the parent environment (which is stateful).
func MapLookuper(m map[string]string) Lookuper {
	return mapLookuper(m)
}

type multiLookuper struct {
	ls []Lookuper
}

var _ Lookuper = (*multiLookuper)(nil)

func (m *multiLookuper) Lookup(key string) (string, bool) {
	for _, l := range m.ls {
		if v, ok := l.Lookup(key); ok {
			return v, true
		}
	}
	return "", false
}

// MultiLookuper wraps a collection of lookupers. It does not combine them, and
// lookups appear in the order in which they are provided to the initializer.
func MultiLookuper(lookupers ...Lookuper) Lookuper {
	return &multiLookuper{ls: lookupers}
}

// Decoder is an interface that custom types/fields can implement to control how
// decoding takes place. For example:
//
//     type MyType string
//
//     func (mt MyType) EnvDecode(val string) error {
//         return "CUSTOM-"+val
//     }
//
type Decoder interface {
	EnvDecode(val string) error
}

// MutatorFunc is a function that mutates a given value before it is passed
// along for processing. This is useful if you want to mutate the environment
// variable value before it's converted to the proper type.
type MutatorFunc func(ctx context.Context, k, v string) (string, error)

// options are internal options for decoding.
type options struct {
	Default  string
	Required bool
}

// ProcessWith processes the given interface with the given lookuper. See the
// package-level documentation for specific examples and behaviors.
func ProcessWith(ctx context.Context, i interface{}, l Lookuper, fns ...MutatorFunc) error {
	if l == nil {
		return ErrLookuperNil
	}

	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Ptr {
		return ErrNotPtr
	}

	e := v.Elem()
	if e.Kind() != reflect.Struct {
		return ErrNotStruct
	}

	t := e.Type()

	for i := 0; i < t.NumField(); i++ {
		ef := e.Field(i)
		tf := t.Field(i)
		tag := tf.Tag.Get(envTag)

		if !ef.CanSet() {
			if tag != "" {
				// There's an "env" tag on a private field, we can't alter it, and it's
				// likely a mistake. Return an error so the user can handle.
				return fmt.Errorf("%s: %w", tf.Name, ErrPrivateField)
			}

			// Otherwise continue to the next field.
			continue
		}

		// Parse the key and options.
		key, opts, err := keyAndOpts(tag)
		if err != nil {
			return fmt.Errorf("%s: %w", tf.Name, err)
		}

		// Initialize pointer structs.
		for ef.Kind() == reflect.Ptr {
			if ef.IsNil() {
				if ef.Type().Elem().Kind() != reflect.Struct {
					// This is a nil pointer to something that isn't a struct, like
					// *string. Move along.
					break
				}

				// Nil pointer to a struct, create so we can traverse.
				ef.Set(reflect.New(ef.Type().Elem()))
			}

			ef = ef.Elem()
		}

		// Special case handle structs. This has to come after the value resolution in
		// case the struct has a custom decoder.
		if ef.Kind() == reflect.Struct {
			for ef.CanAddr() {
				ef = ef.Addr()
			}

			// Lookup the value, ignoring an error if the key isn't defined. This is
			// required for nested structs that don't declare their own `env` keys,
			// but have internal fields with an `env` defined.
			val, err := lookup(key, opts, l)
			if err != nil && !errors.Is(err, ErrMissingKey) {
				return fmt.Errorf("%s: %w", tf.Name, err)
			}

			if err, ok := processAsDecoder(val, ef); ok && err != nil {
				return err
			}

			if err := ProcessWith(ctx, ef.Interface(), l); err != nil {
				return fmt.Errorf("%s: %w", tf.Name, err)
			}

			continue
		}

		// Stop processing if there's no env tag (this comes after nested parsing),
		// in case there's an env tag in an embedded struct.
		if tag == "" {
			continue
		}

		// The field already has a non-zero value, do not overwrite.
		if !ef.IsZero() {
			continue
		}

		val, err := lookup(key, opts, l)
		if err != nil {
			return fmt.Errorf("%s: %w", tf.Name, err)
		}

		// Apply any mutators. Mutators are applied after the lookup, but before any
		// type conversions. They always resolve to a string (or error)
		for _, fn := range fns {
			if fn != nil {
				val, err = fn(ctx, key, val)
				if err != nil {
					return fmt.Errorf("%s: %w", tf.Name, err)
				}
			}
		}

		// Set value.
		if err := processField(val, ef); err != nil {
			return fmt.Errorf("%s(%q): %w", tf.Name, val, err)
		}
	}

	return nil
}

// keyAndOpts parses the given tag value (e.g. env:"foo,required") and
// returns the key name and options as a list.
func keyAndOpts(tag string) (string, *options, error) {
	parts := strings.Split(tag, ",")
	key, tagOpts := strings.TrimSpace(parts[0]), parts[1:]

	var opts options
	for _, o := range tagOpts {
		o = strings.TrimSpace(o)
		switch {
		case o == "required":
			opts.Required = true
		case strings.HasPrefix(o, "default="):
			opts.Default = strings.TrimPrefix(o, "default=")
		default:
			return "", nil, fmt.Errorf("%q: %w", o, ErrUnknownOption)
		}
	}

	return key, &opts, nil
}

// lookup looks up the given key using the provided Lookuper and options.
func lookup(key string, opts *options, l Lookuper) (string, error) {
	if key == "" {
		// The struct has something like `env:",required"`, which is likely a
		// mistake. We could try to infer the envvar from the field name, but that
		// feels too magical.
		return "", ErrMissingKey
	}

	if opts.Required && opts.Default != "" {
		// Having a default value on a required value doesn't make sense.
		return "", ErrRequiredAndDefault
	}

	// Lookup value.
	val, ok := l.Lookup(key)
	if !ok {
		if opts.Required {
			return "", ErrMissingRequired
		}

		if opts.Default != "" {
			val = opts.Default
		}
	}

	// Expand value. This allows for a value or default value that maps to a
	// different variable.
	val = os.Expand(val, func(i string) string {
		s, ok := l.Lookup(i)
		if ok {
			return s
		}
		return i
	})

	return val, nil
}

// processAsDecoder processes the given value as a decoder or custom
// unmarshaller.
func processAsDecoder(v string, ef reflect.Value) (error, bool) {
	if ef.CanInterface() {
		iface := ef.Interface()
		if dec, ok := iface.(Decoder); ok {
			return dec.EnvDecode(v), true
		}

		if tu, ok := iface.(encoding.BinaryUnmarshaler); ok {
			return tu.UnmarshalBinary([]byte(v)), true
		}

		if tu, ok := iface.(encoding.TextUnmarshaler); ok {
			return tu.UnmarshalText([]byte(v)), true
		}
	}

	return nil, false
}

func processField(v string, ef reflect.Value) error {
	// Handle pointers and uninitialized pointers.
	for ef.Type().Kind() == reflect.Ptr {
		if ef.IsNil() {
			ef.Set(reflect.New(ef.Type().Elem()))
		}
		ef = ef.Elem()
	}

	tf := ef.Type()
	tk := tf.Kind()

	// Handle existing decoders.
	if err, ok := processAsDecoder(v, ef); ok && err != nil {
		return err
	}

	// We don't check if the value is empty earlier, because the user might want
	// to define a custom decoder and treat the empty variable as a special case.
	// However, if we got this far, none of the remaining parsers will succeed, so
	// bail out now.
	if v == "" {
		return nil
	}

	switch tk {
	case reflect.Bool:
		b, err := strconv.ParseBool(v)
		if err != nil {
			return err
		}
		ef.SetBool(b)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(v, tf.Bits())
		if err != nil {
			return err
		}
		ef.SetFloat(f)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		i, err := strconv.ParseInt(v, 0, tf.Bits())
		if err != nil {
			return err
		}
		ef.SetInt(i)
	case reflect.Int64:
		// Special case time.Duration values.
		if tf.PkgPath() == "time" && tf.Name() == "Duration" {
			d, err := time.ParseDuration(v)
			if err != nil {
				return err
			}
			ef.SetInt(int64(d))
		} else {
			i, err := strconv.ParseInt(v, 0, tf.Bits())
			if err != nil {
				return err
			}
			ef.SetInt(i)
		}
	case reflect.String:
		ef.SetString(v)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		i, err := strconv.ParseUint(v, 0, tf.Bits())
		if err != nil {
			return err
		}
		ef.SetUint(i)

	case reflect.Interface:
		return fmt.Errorf("cannot decode into interfaces")

	// Maps
	case reflect.Map:
		vals := strings.Split(v, ",")
		mp := reflect.MakeMapWithSize(tf, len(vals))
		for _, val := range vals {
			pair := strings.SplitN(val, ":", 2)
			if len(pair) < 2 {
				return fmt.Errorf("%s: %w", val, ErrInvalidMapItem)
			}
			mKey, mVal := strings.TrimSpace(pair[0]), strings.TrimSpace(pair[1])

			k := reflect.New(tf.Key()).Elem()
			if err := processField(mKey, k); err != nil {
				return fmt.Errorf("%s: %w", mKey, err)
			}

			v := reflect.New(tf.Elem()).Elem()
			if err := processField(mVal, v); err != nil {
				return fmt.Errorf("%s: %w", mVal, err)
			}

			mp.SetMapIndex(k, v)
		}
		ef.Set(mp)

	// Slices
	case reflect.Slice:
		// Special case: []byte
		if tf.Elem().Kind() == reflect.Uint8 {
			ef.Set(reflect.ValueOf([]byte(v)))
		} else {
			vals := strings.Split(v, ",")
			s := reflect.MakeSlice(tf, len(vals), len(vals))
			for i, val := range vals {
				val = strings.TrimSpace(val)
				if err := processField(val, s.Index(i)); err != nil {
					return fmt.Errorf("%s: %w", val, err)
				}
			}
			ef.Set(s)
		}
	}

	return nil
}
