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

package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/export"
)

func TestMain(t *testing.T) {
	t.Parallel()

	db, databaseConfig := database.NewTestDatabaseWithConfig(t)

	exportConfig := &export.Config{
		Database: *databaseConfig,
	}

	t.Errorf("\n\n%#v\n\n", exportConfig)

	_ = db

	os.Setenv("DB_NAME", databaseConfig.Name)
	os.Setenv("DB_USER", databaseConfig.User)
	os.Setenv("DB_HOST", databaseConfig.Host)
	os.Setenv("DB_PORT", databaseConfig.Port)
	os.Setenv("DB_SSLMODE", databaseConfig.SSLMode)
	os.Setenv("DB_PASSWORD", databaseConfig.Password)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := realMain(ctx, exportConfig); err != nil {
			t.Fatal(err)
		}
	}()

	time.Sleep(2 * time.Second)
}
