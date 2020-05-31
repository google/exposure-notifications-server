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
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/signing"
	"github.com/google/exposure-notifications-server/internal/storage"
)

// Note: this test relies on environment variables and therefore CANNOT be run
// in parallel.
func TestSetup(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		_, databaseConfig := database.NewTestDatabaseWithConfig(t)
		os.Setenv("DB_NAME", databaseConfig.Name)
		os.Setenv("DB_USER", databaseConfig.User)
		os.Setenv("DB_HOST", databaseConfig.Host)
		os.Setenv("DB_PORT", databaseConfig.Port)
		os.Setenv("DB_SSLMODE", databaseConfig.SSLMode)
		os.Setenv("DB_PASSWORD", databaseConfig.Password)

		config, env, closer, err := doSetup()
		if err != nil {
			t.Fatal(err)
		}
		defer closer()

		// Database
		if got, want := config.Database.Name, databaseConfig.Name; got != want {
			t.Errorf("expected name %v to be %v", got, want)
		}
		if got, want := config.Database.User, databaseConfig.User; got != want {
			t.Errorf("expected user %v to be %v", got, want)
		}
		if got, want := config.Database.SSLMode, databaseConfig.SSLMode; got != want {
			t.Errorf("expected sslmode %v to be %v", got, want)
		}

		// KeyManager
		if got, want := config.KeyManager.KeyManagerType, signing.KeyManagerType("GOOGLE_CLOUD_KMS"); got != want {
			t.Errorf("expected keymanagertype %v to be %v", got, want)
		}

		// SecretManager
		if got, want := config.SecretManager.SecretManagerType, secrets.SecretManagerType("GOOGLE_SECRET_MANAGER"); got != want {
			t.Errorf("expected secretmanager %v to be %v", got, want)
		}

		// Storage
		if got, want := config.Storage.BlobstoreType, storage.BlobstoreType("GOOGLE_CLOUD_STORAGE"); got != want {
			t.Errorf("expected storage %v to be %v", got, want)
		}

		if env.Blobstore() == nil {
			t.Errorf("expected blobstore")
		}

		if env.Database() == nil {
			t.Errorf("expected database")
		}

		if env.KeyManager() == nil {
			t.Errorf("expected keymanager")
		}

		if env.SecretManager() == nil {
			t.Errorf("expected secretmanager")
		}
	})
}

func TestServer_Run(t *testing.T) {
	t.Parallel()

	// TODO(sethvargo): don't rely on envvar parsing for this test - construct the
	// serverenv and config objects in advance.
	_, databaseConfig := database.NewTestDatabaseWithConfig(t)
	os.Setenv("DB_NAME", databaseConfig.Name)
	os.Setenv("DB_USER", databaseConfig.User)
	os.Setenv("DB_HOST", databaseConfig.Host)
	os.Setenv("DB_PORT", databaseConfig.Port)
	os.Setenv("DB_SSLMODE", databaseConfig.SSLMode)
	os.Setenv("DB_PASSWORD", databaseConfig.Password)

	config, env, closer, err := doSetup()
	if err != nil {
		t.Fatal(err)
	}
	defer closer()

	ctx := context.Background()
	srv, err := NewServer(ctx, config, env)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := srv.Stop(); err != nil {
			t.Fatal(err)
		}
	}()

	go srv.Run()

	// TODO(sethvargo): more robust check
	time.Sleep(500 * time.Millisecond)

	func() {
		client := &http.Client{Timeout: 5 * time.Second}
		u := fmt.Sprintf("http://%s/create-batches", srv.srv.Addr)
		resp, err := client.Get(u)
		if err != nil {
			t.Error(err)
		}

		t.Errorf("\n\n%#v\n\n", resp)
	}()

	func() {
		client := &http.Client{Timeout: 5 * time.Second}
		u := fmt.Sprintf("http://%s/do-work", srv.srv.Addr)
		resp, err := client.Get(u)
		if err != nil {
			t.Error(err)
		}

		t.Errorf("\n\n%#v\n\n", resp)
	}()

}
