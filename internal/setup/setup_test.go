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

package setup_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/authorizedapp"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/internal/signing"
	"github.com/google/exposure-notifications-server/internal/storage"
)

type testDatabaseConfig struct {
	database *database.Config
}

func (t *testDatabaseConfig) DatabaseConfig() *database.Config {
	return t.database
}

func (t *testDatabaseConfig) SecretManagerConfig() *secrets.Config {
	return &secrets.Config{
		SecretManagerType: secrets.SecretManagerType("NOOP"),
		SecretCacheTTL:    10 * time.Minute,
	}
}

type testSecretManagerConfig struct {
	database *database.Config
}

func (t *testSecretManagerConfig) DatabaseConfig() *database.Config {
	return t.database
}

func (t *testSecretManagerConfig) SecretManagerConfig() *secrets.Config {
	return &secrets.Config{
		SecretManagerType: secrets.SecretManagerType("NOOP"),
		SecretCacheTTL:    10 * time.Minute,
	}
}

type testKeyManagerConfig struct {
	database *database.Config
}

func (t *testKeyManagerConfig) DatabaseConfig() *database.Config {
	return t.database
}

func (t *testKeyManagerConfig) KeyManagerConfig() *signing.Config {
	return &signing.Config{
		KeyManagerType: signing.KeyManagerType("NOOP"),
	}
}

func (t *testKeyManagerConfig) SecretManagerConfig() *secrets.Config {
	return &secrets.Config{
		SecretManagerType: secrets.SecretManagerType("NOOP"),
		SecretCacheTTL:    time.Nanosecond,
	}
}

type testBlobstoreConfig struct {
	database *database.Config
}

func (t *testBlobstoreConfig) DatabaseConfig() *database.Config {
	return t.database
}

func (t *testBlobstoreConfig) BlobstoreConfig() *storage.Config {
	return &storage.Config{
		BlobstoreType: storage.BlobstoreType("NOOP"),
	}
}

type testAuthorizedAppConfig struct {
	database *database.Config
}

func (t *testAuthorizedAppConfig) DatabaseConfig() *database.Config {
	return t.database
}

func (t *testAuthorizedAppConfig) AuthorizedAppConfig() *authorizedapp.Config {
	return &authorizedapp.Config{}
}

func TestSetup(t *testing.T) {
	t.Parallel()

	t.Run("default", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		_, dbconfig := database.NewTestDatabaseWithConfig(t)
		config := &testDatabaseConfig{database: dbconfig}
		env, done, err := setup.Setup(ctx, config)
		if err != nil {
			t.Fatal(err)
		}
		defer done()

		db := env.Database()
		if db == nil {
			t.Errorf("expected db to exist")
		}
	})

	t.Run("secret_manager", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		_, dbconfig := database.NewTestDatabaseWithConfig(t)
		config := &testSecretManagerConfig{database: dbconfig}
		env, done, err := setup.Setup(ctx, config)
		if err != nil {
			t.Fatal(err)
		}
		defer done()

		sm := env.SecretManager()
		if sm == nil {
			t.Errorf("expected secret manager to exist")
		}

		if _, ok := sm.(*secrets.Cacher); !ok {
			t.Errorf("expected %T to be Cacher", sm)
		}
	})

	t.Run("key_manager", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		_, dbconfig := database.NewTestDatabaseWithConfig(t)
		config := &testKeyManagerConfig{database: dbconfig}
		env, done, err := setup.Setup(ctx, config)
		if err != nil {
			t.Fatal(err)
		}
		defer done()

		km := env.KeyManager()
		if km == nil {
			t.Errorf("expected key manager to exist")
		}

		if _, ok := km.(*signing.Noop); !ok {
			t.Errorf("expected %T to be Noop", km)
		}
	})

	t.Run("blobstore", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		_, dbconfig := database.NewTestDatabaseWithConfig(t)
		config := &testBlobstoreConfig{database: dbconfig}
		env, done, err := setup.Setup(ctx, config)
		if err != nil {
			t.Fatal(err)
		}
		defer done()

		bs := env.Blobstore()
		if bs == nil {
			t.Errorf("expected blobstore to exist")
		}

		if _, ok := bs.(*storage.Noop); !ok {
			t.Errorf("expected %T to be Noop", bs)
		}
	})

	t.Run("authorizedapp", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		_, dbconfig := database.NewTestDatabaseWithConfig(t)
		config := &testAuthorizedAppConfig{database: dbconfig}
		env, done, err := setup.Setup(ctx, config)
		if err != nil {
			t.Fatal(err)
		}
		defer done()

		ap := env.AuthorizedAppProvider()
		if ap == nil {
			t.Errorf("expected appprovider to exist")
		}

		if _, ok := ap.(*authorizedapp.DatabaseProvider); !ok {
			t.Errorf("expected %T to be DatabaseProvider", ap)
		}
	})
}
