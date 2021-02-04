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

package project

import (
	"context"
	"testing"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// TestContext returns a context with test values pre-populated.
func TestContext(tb testing.TB) context.Context {
	ctx := context.Background()
	ctx = logging.WithLogger(ctx, TestLogger(tb))
	return ctx
}

// TestLogger returns a logger configured for test. See the following link for
// more information:
//
//     https://pkg.go.dev/go.uber.org/zap/zaptest
//
func TestLogger(tb testing.TB) *zap.SugaredLogger {
	return zaptest.NewLogger(tb, zaptest.Level(zap.WarnLevel)).Sugar()
}
