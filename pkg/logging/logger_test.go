// Copyright 2020 the Exposure Notifications Server authors
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

package logging

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap/zapcore"
)

func TestNewLogger(t *testing.T) {
	t.Parallel()

	logger := NewLogger("", true)
	if logger == nil {
		t.Fatal("expected logger to never be nil")
	}
}

func TestDefaultLogger(t *testing.T) {
	t.Parallel()

	logger1 := DefaultLogger()
	if logger1 == nil {
		t.Fatal("expected logger to never be nil")
	}

	logger2 := DefaultLogger()
	if logger2 == nil {
		t.Fatal("expected logger to never be nil")
	}

	// Intentionally comparing identities here
	if logger1 != logger2 {
		t.Errorf("expected %#v to be %#v", logger1, logger2)
	}
}

func TestContext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger1 := FromContext(ctx)
	if logger1 == nil {
		t.Fatal("expected logger to never be nil")
	}

	ctx = WithLogger(ctx, logger1)

	logger2 := FromContext(ctx)
	if logger1 != logger2 {
		t.Errorf("expected %#v to be %#v", logger1, logger2)
	}
}

func TestLevelToZapLevls(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  zapcore.Level
	}{
		{input: levelDebug, want: zapcore.DebugLevel},
		{input: levelInfo, want: zapcore.InfoLevel},
		{input: levelWarning, want: zapcore.WarnLevel},
		{input: levelError, want: zapcore.ErrorLevel},
		{input: levelCritical, want: zapcore.DPanicLevel},
		{input: levelAlert, want: zapcore.PanicLevel},
		{input: levelEmergency, want: zapcore.FatalLevel},
		{input: "unknown", want: zapcore.WarnLevel},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()

			got := levelToZapLevel(tc.input)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
