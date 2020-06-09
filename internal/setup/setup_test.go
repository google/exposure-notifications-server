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
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/authorizedapp"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/internal/signing"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/sethvargo/go-envconfig/pkg/envconfig"
)

var _ setup.AuthorizedAppConfigProvider = (*testConfig)(nil)
var _ setup.BlobstoreConfigProvider = (*testConfig)(nil)
var _ setup.DatabaseConfigProvider = (*testConfig)(nil)
var _ setup.KeyManagerConfigProvider = (*testConfig)(nil)
var _ setup.SecretManagerConfigProvider = (*testConfig)(nil)

type testConfig struct {
	Database   *database.Config
	Secret     string `env:"MY_SECRET"`
	SecretFile string `env:"MY_SECRET_FILE"`
}

func (t *testConfig) AuthorizedAppConfig() *authorizedapp.Config {
	return &authorizedapp.Config{
		// TODO: type
		CacheDuration: 10 * time.Minute,
	}
}

func (t *testConfig) BlobstoreConfig() *storage.Config {
	return &storage.Config{
		BlobstoreType: storage.BlobstoreType("NOOP"),
	}
}

func (t *testConfig) DatabaseConfig() *database.Config {
	return t.Database
}

func (t *testConfig) KeyManagerConfig() *signing.Config {
	return &signing.Config{
		KeyManagerType: signing.KeyManagerType("NOOP"),
	}
}

func (t *testConfig) SecretManagerConfig() *secrets.Config {
	return &secrets.Config{
		SecretManagerType: secrets.SecretManagerType("NOOP"),
		SecretCacheTTL:    10 * time.Minute,
	}
}

func TestSetupWith(t *testing.T) {
	t.Parallel()

	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	lookuper := envconfig.MapLookuper(map[string]string{
		"SECRETS_DIR":    tmp,
		"MY_SECRET":      "secret://foo",
		"MY_SECRET_FILE": "secret://foo?target=file",
	})

	t.Run("default", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		_, dbconfig := database.NewTestDatabaseWithConfig(t)

		config := &testConfig{Database: dbconfig}
		env, err := setup.SetupWith(ctx, config, lookuper)
		if err != nil {
			t.Fatal(err)
		}
		defer env.Close(ctx)

		if got, want := config.Secret, "noop-secret"; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
	})

	t.Run("database", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		_, dbconfig := database.NewTestDatabaseWithConfig(t)

		config := &testConfig{Database: dbconfig}
		env, err := setup.SetupWith(ctx, config, lookuper)
		if err != nil {
			t.Fatal(err)
		}
		defer env.Close(ctx)

		db := env.Database()
		if db == nil {
			t.Errorf("expected db to exist")
		}
	})

	t.Run("authorizedapp", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		_, dbconfig := database.NewTestDatabaseWithConfig(t)

		config := &testConfig{Database: dbconfig}
		env, err := setup.SetupWith(ctx, config, lookuper)
		if err != nil {
			t.Fatal(err)
		}
		defer env.Close(ctx)

		ap := env.AuthorizedAppProvider()
		if ap == nil {
			t.Errorf("expected appprovider to exist")
		}

		if _, ok := ap.(*authorizedapp.DatabaseProvider); !ok {
			t.Errorf("expected %T to be DatabaseProvider", ap)
		}
	})

	t.Run("blobstore", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		_, dbconfig := database.NewTestDatabaseWithConfig(t)

		config := &testConfig{Database: dbconfig}
		env, err := setup.SetupWith(ctx, config, lookuper)
		if err != nil {
			t.Fatal(err)
		}
		defer env.Close(ctx)

		bs := env.Blobstore()
		if bs == nil {
			t.Errorf("expected blobstore to exist")
		}

		if _, ok := bs.(*storage.Noop); !ok {
			t.Errorf("expected %T to be Noop", bs)
		}
	})

	t.Run("key_manager", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		_, dbconfig := database.NewTestDatabaseWithConfig(t)

		config := &testConfig{Database: dbconfig}
		env, err := setup.SetupWith(ctx, config, lookuper)
		if err != nil {
			t.Fatal(err)
		}
		defer env.Close(ctx)

		km := env.KeyManager()
		if km == nil {
			t.Errorf("expected key manager to exist")
		}

		if _, ok := km.(*signing.Noop); !ok {
			t.Errorf("expected %T to be Noop", km)
		}
	})

	t.Run("secret_manager", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		_, dbconfig := database.NewTestDatabaseWithConfig(t)

		config := &testConfig{Database: dbconfig}
		env, err := setup.SetupWith(ctx, config, lookuper)
		if err != nil {
			t.Fatal(err)
		}
		defer env.Close(ctx)

		sm := env.SecretManager()
		if sm == nil {
			t.Errorf("expected secret manager to exist")
		}

		if _, ok := sm.(*secrets.Cacher); !ok {
			t.Errorf("expected %T to be Cacher", sm)
		}

		if got, want := config.Secret, "noop-secret"; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}

		if got, want := config.SecretFile, tmp; !strings.Contains(got, want) {
			t.Errorf("expected %v to contain %v", got, want)
		}
	})
}
