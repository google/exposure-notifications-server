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

package enkstest

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/authorizedapp"
	"github.com/google/exposure-notifications-server/internal/cleanup"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/export"
	"github.com/google/exposure-notifications-server/internal/federationin"
	"github.com/google/exposure-notifications-server/internal/keyrotation"
	"github.com/google/exposure-notifications-server/internal/middleware"
	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/internal/publish"
	"github.com/google/exposure-notifications-server/internal/revision"
	revisiondatabase "github.com/google/exposure-notifications-server/internal/revision/database"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/google/exposure-notifications-server/pkg/server"

	"github.com/gorilla/mux"
	"github.com/sethvargo/go-envconfig"
)

type Server struct {
	Server *server.Server
	Env    *serverenv.ServerEnv
}

// NewServer sets up local servers for running tests. The server runs on a
// single address with different subpaths for each service.
func NewServer(tb testing.TB, testDatabaseInstance *database.TestInstance) *Server {
	tb.Helper()

	// Do not start the server when running short tests.
	if testing.Short() {
		tb.SkipNow()
	}

	ctx := project.TestContext(tb)

	blobstore, err := storage.NewMemory(ctx, &storage.Config{})
	if err != nil {
		tb.Fatal(err)
	}

	kms := keys.TestKeyManager(tb)

	secretManager, err := secrets.NewInMemory(ctx, &secrets.Config{})
	if err != nil {
		tb.Fatal(err)
	}

	db, _ := testDatabaseInstance.NewDatabase(tb)

	aap, err := authorizedapp.NewDatabaseProvider(ctx, db, &authorizedapp.Config{
		CacheDuration: time.Nanosecond,
	})
	if err != nil {
		tb.Fatal(err)
	}

	tokenKey := keys.TestEncryptionKey(tb, kms)

	// create an initial revision key.
	revisionDB, err := revisiondatabase.New(db, &revisiondatabase.KMSConfig{
		KeyManager:   kms,
		WrapperKeyID: tokenKey,
	})
	if err != nil {
		tb.Fatal(err)
	}
	if _, err := revisionDB.CreateRevisionKey(ctx); err != nil {
		tb.Fatal(err)
	}

	env := serverenv.New(ctx,
		serverenv.WithAuthorizedAppProvider(aap),
		serverenv.WithBlobStorage(blobstore),
		serverenv.WithDatabase(db),
		serverenv.WithKeyManager(kms),
		serverenv.WithSecretManager(secretManager),
	)
	// Note: don't call env.Cleanup() because the database helper closes the
	// connection for us.
	r := mux.NewRouter()

	// cleanup-export
	cleanupExportConfig := &cleanup.Config{
		Timeout: 10 * time.Minute,
		TTL:     336 * time.Hour,
	}
	processDefaults(tb, cleanupExportConfig)
	cleanupExportHandler, err := cleanup.NewExportHandler(cleanupExportConfig, env)
	if err != nil {
		tb.Fatal(err)
	}
	r.Handle("/cleanup-export", cleanupExportHandler)

	// cleanup-exposure
	cleanupExposureConfig := &cleanup.Config{
		Timeout: 10 * time.Minute,
		TTL:     336 * time.Hour,
	}
	processDefaults(tb, cleanupExposureConfig)
	cleanupExposureHandler, err := cleanup.NewExposureHandler(cleanupExposureConfig, env)
	if err != nil {
		tb.Fatal(err)
	}
	r.Handle("/cleanup-exposure", cleanupExposureHandler)

	// export
	exportConfig := &export.Config{
		CreateTimeout:  10 * time.Second,
		WorkerTimeout:  10 * time.Second,
		MinRecords:     1,
		PaddingRange:   1,
		MaxRecords:     100,
		TruncateWindow: 1 * time.Second,
		MinWindowAge:   1 * time.Second,
		TTL:            336 * time.Hour,
	}
	processDefaults(tb, exportConfig)
	exportServer, err := export.NewServer(exportConfig, env)
	if err != nil {
		tb.Fatal(err)
	}
	r.PathPrefix("/export/").Handler(http.StripPrefix("/export", exportServer.Routes(ctx)))

	// federationin
	federationInConfig := &federationin.Config{
		Timeout:        10 * time.Minute,
		TruncateWindow: 1 * time.Hour,
	}
	processDefaults(tb, federationInConfig)
	federationInServer, err := federationin.NewServer(federationInConfig, env)
	if err != nil {
		tb.Fatal(err)
	}
	r.PathPrefix("/federation-in/").Handler(http.StripPrefix("/federation-in", federationInServer.Routes(ctx)))

	// Federation out
	// TODO: this is a grpc listener and requires a lot of setup.

	// key-rotation
	keyRotationConfig := &keyrotation.Config{
		RevisionToken: revision.Config{
			KeyID:     tokenKey,
			AAD:       []byte{1, 2, 3},
			MinLength: 28,
		},
		NewKeyPeriod:       100 * time.Millisecond,
		DeleteOldKeyPeriod: 100 * time.Millisecond,
	}
	processDefaults(tb, keyRotationConfig)
	rotationServer, err := keyrotation.NewServer(keyRotationConfig, env)
	if err != nil {
		tb.Fatal(err)
	}
	r.PathPrefix("/key-rotation/").Handler(http.StripPrefix("/key-rotation", rotationServer.Routes(ctx)))

	// publish
	publishConfig := &publish.Config{}
	processDefaults(tb, publishConfig)
	publishConfig.RevisionToken = keyRotationConfig.RevisionToken
	publishConfig.MaxKeysOnPublish = 15
	publishConfig.MaxSameStartIntervalKeys = 2
	publishConfig.MaxIntervalAge = 360 * time.Hour
	publishConfig.CreatedAtTruncateWindow = time.Second
	publishConfig.ReleaseSameDayKeys = true
	publishConfig.RevisionKeyCacheDuration = time.Second

	publishServer, err := publish.NewServer(ctx, publishConfig, env)
	if err != nil {
		tb.Fatal(err)
	}
	r.PathPrefix("/publish/").Handler(http.StripPrefix("/publish", publishServer.Routes(ctx)))

	// Inject the test logger into the context instead of the default sugared
	// logger.
	mux := middleware.PopulateLogger(project.TestLogger(tb))(r)

	// Create a stoppable context.
	doneCtx, cancel := context.WithCancel(context.Background())
	tb.Cleanup(func() {
		cancel()
	})

	// As of 2020-10-29, our CI infrastructure does not support IPv6. `server.New`
	// binds to "tcp", which picks the "best" address, but it prefers IPv6. As a
	// result, the server binds to the IPv6 loopback`[::]`, but then our browser
	// instance cannot actually contact that loopback interface. To mitigate this,
	// create a custom listener and force IPv4. The listener will still pick a
	// randomly available port, but it will only choose an IPv4 address upon which
	// to bind.
	listener, err := net.Listen("tcp4", ":0")
	if err != nil {
		tb.Fatalf("failed to create listener: %v", err)
	}

	// Start the server on a random port. Closing doneCtx will stop the server
	// (which the cleanup step does).
	srv, err := server.NewFromListener(listener)
	if err != nil {
		tb.Fatal(err)
	}
	go func() {
		if err := srv.ServeHTTPHandler(doneCtx, mux); err != nil {
			tb.Error(err)
		}
	}()

	return &Server{
		Server: srv,
		Env:    env,
	}
}

func processDefaults(tb testing.TB, i interface{}) {
	tb.Helper()

	if err := envconfig.ProcessWith(context.Background(), i, envconfig.MapLookuper(map[string]string{})); err != nil {
		tb.Fatal(err)
	}
}
