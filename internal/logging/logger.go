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

// Package logging sets up and configures logging.
package logging

import (
	"context"

	"go.uber.org/zap"
)

type loggerKey struct{}

var fallbackLogger *zap.SugaredLogger

func init() {
	// Set up Stackdriver keys (see: https://cloud.google.com/run/docs/logging#special-fields)
	config := zap.NewProductionConfig()
	config.EncoderConfig.MessageKey = "message"
	config.EncoderConfig.LevelKey = "severity"
	config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)

	if logger, err := config.Build(); err != nil {
		fallbackLogger = zap.NewNop().Sugar()
	} else {
		fallbackLogger = logger.Named("default").Sugar()
	}
}

func WithLogger(ctx context.Context, logger *zap.SugaredLogger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

func FromContext(ctx context.Context) *zap.SugaredLogger {
	if logger, ok := ctx.Value(loggerKey{}).(*zap.SugaredLogger); ok {
		return logger
	}
	return fallbackLogger
}
