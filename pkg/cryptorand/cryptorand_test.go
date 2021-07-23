// Copyright 2021 the Exposure Notifications Server authors
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

package cryptorand

import (
	"testing"
)

func TestNewSource(t *testing.T) {
	t.Parallel()
	NewSource()
}

func TestSrc_Seed(t *testing.T) {
	t.Parallel()

	rnd := NewSource()
	typ1 := rnd.(*src)
	typ2 := typ1

	typ1.Seed(0)
	if typ1 != typ2 {
		t.Errorf("seeding should not change struct")
	}
}

func TestSrc_Int63(t *testing.T) {
	t.Parallel()

	n := 50
	found := make(map[int64]struct{}, n)

	rnd := NewSource()
	for i := 0; i < n; i++ {
		v := rnd.Int63()
		if _, ok := found[v]; ok {
			t.Errorf("not random (value %d already exists)", v)
		}
	}
}

func TestSrc_Uint64(t *testing.T) {
	t.Parallel()

	n := 50
	found := make(map[uint64]struct{}, n)

	rnd := NewSource()
	for i := 0; i < n; i++ {
		v := rnd.Uint64()
		if _, ok := found[v]; ok {
			t.Errorf("not random (value %d already exists)", v)
		}
	}
}
