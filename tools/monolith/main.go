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

// Package main runs all the server components at different URL paths.
package main

import (
	"github.com/google/exposure-notifications-server/internal/interrupt"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/monolith"

	// Enable observability with distributed tracing and metrics.
	_ "github.com/google/exposure-notifications-server/internal/observability"
)

func main() {
	ctx, done := interrupt.Context()
	defer done()

	if err := monolith.RunServer(ctx); err != nil {
		logger := logging.FromContext(ctx)
		logger.Fatal(err)
	}
}
