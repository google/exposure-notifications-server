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

// Package cryptorand defines a rand.Source from crypto/rand.
package cryptorand

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	mrand "math/rand"
)

// Compile time type check.
var _ mrand.Source64 = (*src)(nil)

// NewSource returns a new math/rand.Source that uses crypto/rand as the random
// generation.
func NewSource() mrand.Source64 {
	return new(src)
}

type src struct{}

func (s *src) Seed(seed int64) {}

func (s *src) Int63() int64 {
	return int64(s.Uint64() & ^uint64(1<<63))
}

func (s *src) Uint64() uint64 {
	var v uint64
	if err := binary.Read(crand.Reader, binary.BigEndian, &v); err != nil {
		panic(fmt.Sprintf("failed to read random: %v", err))
	}
	return v
}
