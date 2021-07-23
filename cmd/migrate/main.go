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

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/google/exposure-notifications-server/internal/buildinfo"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/logging"
)

var (
	pathFlag         = flag.String("path", "migrations/", "path to migrations folder")
	migrationTimeout = flag.Duration("timeout", 15*time.Minute, "duration for migration timeout")
)

func main() {
	flag.Parse()

	ctx, done := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	logger := logging.NewLoggerFromEnv().
		With("build_id", buildinfo.BuildID).
		With("build_tag", buildinfo.BuildTag)
	ctx = logging.WithLogger(ctx, logger)

	defer func() {
		done()
		if r := recover(); r != nil {
			logger.Fatalw("application panic", "panic", r)
		}
	}()

	err := realMain(ctx)
	done()

	if err != nil {
		log.Fatal(err)
	}
	logger.Info("successful shutdown")
}

func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	var config database.Config
	env, err := setup.Setup(ctx, &config)
	if err != nil {
		return fmt.Errorf("failed to setup database: %w", err)
	}
	defer env.Close(ctx)

	// Run the migrations
	dir := fmt.Sprintf("file://%s", *pathFlag)
	m, err := migrate.New(dir, config.ConnectionURL())
	if err != nil {
		return fmt.Errorf("failed create migrate: %w", err)
	}
	m.LockTimeout = *migrationTimeout
	m.Log = newLogger()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed run migrate: %w", err)
	}
	srcErr, dbErr := m.Close()
	if srcErr != nil {
		return fmt.Errorf("migrate source error: %w", srcErr)
	}
	if dbErr != nil {
		return fmt.Errorf("migrate database error: %w", dbErr)
	}

	logger.Debugw("finished running migrations")
	return nil
}

type logger struct {
	logger *log.Logger
}

func newLogger() *logger {
	return &logger{
		logger: log.New(os.Stdout, "migrate", log.LstdFlags),
	}
}

func (l *logger) Printf(arg string, vars ...interface{}) {
	l.logger.Printf(arg, vars...)
}

func (l *logger) Verbose() bool {
	return true
}
