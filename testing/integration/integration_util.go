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

package integration

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/monolith"
)

type SystemUnderTest struct {
	Export  string
	Publish string
}

func StartSystemUnderTest(tb testing.TB, ctx context.Context) *SystemUnderTest {
	tb.Helper()

	if testing.Short() {
		tb.Skipf("🚧 Skipping integration tests (short!")
	}

	if skip, _ := strconv.ParseBool(os.Getenv("SKIP_INTEGRATION_TESTS")); skip {
		tb.Skipf("🚧 Skipping integration tests (SKIP_INTEGRATION_TESTS is set)!")
	}

	database.NewTestDatabase(tb)

	monolith.RunServer(ctx)

	sut := &SystemUnderTest{"http://localhost:80/do-work", "http://localhost:80/publish"}
	return sut

}
