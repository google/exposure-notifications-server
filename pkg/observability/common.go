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

package observability

import (
	"context"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
)

// This file defines some common variables used by both the key server and
// verification server.

var (
	// BlameTagKey indicating Who to blame for the API request failure.
	// NONE: no failure
	// CLIENT: the client is at fault (e.g. invalid request)
	// SERVER: the server is at fault
	// EXTERNAL: some third party is at fault
	// UNKNOWN: for everything else
	BlameTagKey = tag.MustNewKey("blame")

	// ResultTagKey contains a free format text describing the result of the
	// request. Preferably ALL CAPS WITH UNDERSCORE.
	// OK indicating a successful request.
	// You can losely base this string on
	// https://github.com/googleapis/googleapis/blob/master/google/rpc/code.proto
	// but feel free to use any text as long as it's easy to filter.
	ResultTagKey = tag.MustNewKey("result")
)

var (
	// BlameNone indicate no API failure
	BlameNone = tag.Upsert(BlameTagKey, "NONE")

	// BlameClient indicate the client is at fault (e.g. invalid request)
	BlameClient = tag.Upsert(BlameTagKey, "CLIENT")

	// BlameServer indicate the server is at fault
	BlameServer = tag.Upsert(BlameTagKey, "SERVER")

	// BlameExternal indicate some third party is at fault
	BlameExternal = tag.Upsert(BlameTagKey, "EXTERNAL")

	// BlameUnknown can be used for everything else
	BlameUnknown = tag.Upsert(BlameTagKey, "UNKNOWN")
)

var (
	// ResultOK add a tag indicating the API call is a success.
	ResultOK = tag.Upsert(ResultTagKey, "OK")
	// ResultNotOK add a tag indicating the API call is a failure.
	ResultNotOK = ResultError("NOT_OK")
)

// ResultError add a tag with the given string as the result.
func ResultError(result string) tag.Mutator {
	return tag.Upsert(ResultTagKey, result)
}

// BuildInfo is the interface to provide build information.
type BuildInfo interface {
	ID() string
	Tag() string
}

// RecordLatency calculate and record the latency.
// Usage example:
// func foo() {
// 	 defer RecordLatency(&ctx, time.Now(), metric, tag1, tag2)
//   // remaining of the function body.
// }
func RecordLatency(ctx context.Context, start time.Time, m *stats.Float64Measure, mutators ...*tag.Mutator) {
	var additionalMutators []tag.Mutator
	for _, t := range mutators {
		additionalMutators = append(additionalMutators, *t)
	}
	// Calculate the millisecond number as float64. time.Duration.Millisecond()
	// returns an integer.
	latency := float64(time.Since(start)) / float64(time.Millisecond)
	stats.RecordWithTags(ctx, additionalMutators, m.M(latency))
}
