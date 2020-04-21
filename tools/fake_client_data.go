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

// Tool to write extreme fake CDN data for client testing.
package main

import (
	"bufio"
	"encoding/binary"
	"io"
	"os"
)

func Write32(writer io.Writer, value uint32) {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data[0:], value)
	n, err := writer.Write(data)
	if n != 4 || err != nil {
		panic("Incomplete Write32")
	}
}

func Write64(writer io.Writer, value uint64) {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data[0:], value)
	n, err := writer.Write(data)
	if n != 8 || err != nil {
		panic("Incomplete Write64")
	}
}

func main() {
	if len(os.Args) != 2 {
		panic("Usage: fake_client_data.go <outfile>")
	}
	f, err := os.Create(os.Args[1])
	if err != nil {
		panic("Cannot open output file")
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	defer w.Flush()

	const day_in_secs = 24 * 60 * 60
	const days_to_write = 14
	const keys_per_subbatch = 10000

	var counter uint64 = 0

	// Write version number 0
	Write32(w, 0)
	// Write days_to_write subbatches (one for each day)
	Write32(w, days_to_write)
	for day := 0; day < days_to_write; day++ {
		// Write incremental day stamp in UTC since epoch
		const feb15_2020 uint64 = 1586908800
		Write64(w, feb15_2020+uint64(day_in_secs*day))
		// Write empty metadata
		Write32(w, 0)
		// Write count for size of subbatch
		Write32(w, keys_per_subbatch)
		for i := 0; i < keys_per_subbatch; i++ {
			// Write unique key for each position
			Write64(w, counter)
			Write64(w, counter)
			counter++
		}
	}
}
