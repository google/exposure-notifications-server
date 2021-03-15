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
	"os"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/authorizedapp"
	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/sethvargo/go-envconfig"
)

var testDatabaseInstance *database.TestInstance

func TestMain(m *testing.M) {
	testDatabaseInstance = database.MustTestInstance()
	defer testDatabaseInstance.MustClose()
	m.Run()
}

var (
	_ setup.AuthorizedAppConfigProvider         = (*testConfig)(nil)
	_ setup.BlobstoreConfigProvider             = (*testConfig)(nil)
	_ setup.DatabaseConfigProvider              = (*testConfig)(nil)
	_ setup.KeyManagerConfigProvider            = (*testConfig)(nil)
	_ setup.SecretManagerConfigProvider         = (*testConfig)(nil)
	_ setup.ObservabilityExporterConfigProvider = (*testConfig)(nil)
)

type testConfig struct {
	Database *database.Config
}

func (t *testConfig) AuthorizedAppConfig() *authorizedapp.Config {
	return &authorizedapp.Config{
		// TODO: type
		CacheDuration: 10 * time.Minute,
	}
}

func (t *testConfig) BlobstoreConfig() *storage.Config {
	return &storage.Config{
		Type: "MEMORY",
	}
}

func (t *testConfig) DatabaseConfig() *database.Config {
	return t.Database
}

func (t *testConfig) KeyManagerConfig() *keys.Config {
	return &keys.Config{
		Type: "FILESYSTEM",
	}
}

func (t *testConfig) SecretManagerConfig() *secrets.Config {
	return &secrets.Config{
		Type:           "IN_MEMORY",
		SecretCacheTTL: 10 * time.Minute,
	}
}

func (t *testConfig) ObservabilityExporterConfig() *observability.Config {
	return &observability.Config{
		ExporterType: observability.ExporterType("NOOP"),
	}
}

func TestSetupWith(t *testing.T) {
	t.Parallel()

	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	lookuper := envconfig.MapLookuper(map[string]string{})

	t.Run("default", func(t *testing.T) {
		t.Parallel()

		ctx := project.TestContext(t)
		_, dbconfig := testDatabaseInstance.NewDatabase(t)

		config := &testConfig{Database: dbconfig}
		env, err := setup.SetupWith(ctx, config, lookuper)
		if err != nil {
			t.Fatal(err)
		}
		defer env.Close(ctx)
	})

	t.Run("database", func(t *testing.T) {
		t.Parallel()

		ctx := project.TestContext(t)
		_, dbconfig := testDatabaseInstance.NewDatabase(t)

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

		ctx := project.TestContext(t)
		_, dbconfig := testDatabaseInstance.NewDatabase(t)

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

		ctx := project.TestContext(t)
		_, dbconfig := testDatabaseInstance.NewDatabase(t)

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

		if _, ok := bs.(*storage.Memory); !ok {
			t.Errorf("expected %T to be Noop", bs)
		}
	})

	t.Run("key_manager", func(t *testing.T) {
		t.Parallel()

		ctx := project.TestContext(t)
		_, dbconfig := testDatabaseInstance.NewDatabase(t)

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

		if _, ok := km.(*keys.Filesystem); !ok {
			t.Errorf("expected %T to be Filesystem", km)
		}
	})

	t.Run("secret_manager", func(t *testing.T) {
		t.Parallel()

		ctx := project.TestContext(t)
		_, dbconfig := testDatabaseInstance.NewDatabase(t)

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
	})

	t.Run("observability_exporter", func(t *testing.T) {
		t.Parallel()

		ctx := project.TestContext(t)
		_, dbconfig := testDatabaseInstance.NewDatabase(t)

		config := &testConfig{Database: dbconfig}
		env, err := setup.SetupWith(ctx, config, lookuper)
		if err != nil {
			t.Fatal(err)
		}
		defer env.Close(ctx)

		oe := env.ObservabilityExporter()
		if oe == nil {
			t.Errorf("expected observability exporter to exist")
		}
		defer func() {
			if err := oe.Close(); err != nil {
				t.Fatal(err)
			}
		}()
	})
}
