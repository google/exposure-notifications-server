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

package integration

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"path"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/authorizedapp"
	"github.com/google/exposure-notifications-server/internal/cleanup"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/export"
	"github.com/google/exposure-notifications-server/internal/federationin"
	"github.com/google/exposure-notifications-server/internal/keyrotation"
	"github.com/google/exposure-notifications-server/internal/publish"
	"github.com/google/exposure-notifications-server/internal/revision"
	revdb "github.com/google/exposure-notifications-server/internal/revision/database"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/storage"
	vm "github.com/google/exposure-notifications-server/internal/verification/model"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/google/exposure-notifications-server/pkg/server"
	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-retry"

	authorizedappdb "github.com/google/exposure-notifications-server/internal/authorizedapp/database"
	authorizedappmodel "github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	exportdatabase "github.com/google/exposure-notifications-server/internal/export/database"
	exportmodel "github.com/google/exposure-notifications-server/internal/export/model"
	testutil "github.com/google/exposure-notifications-server/internal/utils"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
)

// NewTestServer sets up mocked local servers for running tests
func NewTestServer(tb testing.TB) (*serverenv.ServerEnv, *Client) {
	tb.Helper()

	// Do not start the server when running short tests.
	if testing.Short() {
		tb.SkipNow()
	}

	ctx := context.Background()

	var bs storage.Blobstore
	var err error
	if v := os.Getenv("E2E_GOOGLE_CLOUD_BUCKET"); v != "" {
		// Use a real Cloud Storage bucket if this envvar is set.
		bs, err = storage.NewGoogleCloudStorage(ctx)
		if err != nil {
			tb.Fatal(err)
		}
	} else {
		bs, err = storage.NewMemory(ctx)
		if err != nil {
			tb.Fatal(err)
		}
	}

	var db *database.DB
	if v := os.Getenv("E2E_DB_NAME"); v != "" {
		// Use the real database if this envvar is set.
		var dbConfig database.Config
		sm, err := secrets.SecretManagerFor(ctx, secrets.SecretManagerTypeGoogleSecretManager)
		if err != nil {
			tb.Fatalf("unable to connect to secret manager: %v", err)
		}
		if err := envconfig.ProcessWith(ctx, dbConfig, envconfig.OsLookuper(),
			secrets.Resolver(sm, &secrets.Config{})); err != nil {
			tb.Fatalf("error loading environment variables: %v", err)
		}

		db, err = database.NewFromEnv(ctx, &dbConfig)
		if err != nil {
			tb.Fatalf("unable to connect to database: %v", err)
		}
	} else {
		db = database.NewTestDatabase(tb)
	}

	aap, err := authorizedapp.NewDatabaseProvider(ctx, db, &authorizedapp.Config{CacheDuration: time.Nanosecond})
	if err != nil {
		tb.Fatal(err)
	}

	kms := keys.TestKeyManager(tb)
	tokenKey := keys.TestEncryptionKey(tb, kms)

	// create an initial revision key.
	revisionDB, err := revdb.New(db, &revdb.KMSConfig{WrapperKeyID: tokenKey, KeyManager: kms})
	if err != nil {
		tb.Fatal(err)
	}
	if _, err := revisionDB.CreateRevisionKey(ctx); err != nil {
		tb.Fatal(err)
	}

	sm, err := secrets.NewInMemory(ctx)
	if err != nil {
		tb.Fatal(err)
	}

	env := serverenv.New(ctx,
		serverenv.WithAuthorizedAppProvider(aap),
		serverenv.WithBlobStorage(bs),
		serverenv.WithDatabase(db),
		serverenv.WithKeyManager(kms),
		serverenv.WithSecretManager(sm),
	)
	// Note: don't call env.Cleanup() because the database helper closes the
	// connection for us.
	mux := http.NewServeMux()

	// Cleanup export
	cleanupExportConfig := &cleanup.Config{
		Timeout: 10 * time.Minute,
		TTL:     336 * time.Hour,
	}

	cleanupExportHandler, err := cleanup.NewExportHandler(cleanupExportConfig, env)
	if err != nil {
		tb.Fatal(err)
	}
	mux.Handle("/cleanup-export", cleanupExportHandler)

	// Cleanup exposure
	cleanupExposureConfig := &cleanup.Config{
		Timeout: 10 * time.Minute,
		TTL:     336 * time.Hour,
	}

	cleanupExposureHandler, err := cleanup.NewExposureHandler(cleanupExposureConfig, env)
	if err != nil {
		tb.Fatal(err)
	}
	mux.Handle("/cleanup-exposure", cleanupExposureHandler)

	// Export
	exportConfig := &export.Config{
		CreateTimeout:  10 * time.Second,
		WorkerTimeout:  10 * time.Second,
		MinRecords:     1,
		PaddingRange:   1,
		MaxRecords:     10000,
		TruncateWindow: 1 * time.Second,
		MinWindowAge:   1 * time.Second,
		TTL:            336 * time.Hour,
	}

	exportServer, err := export.NewServer(exportConfig, env)
	if err != nil {
		tb.Fatal(err)
	}
	exportHandler := http.StripPrefix("/export", exportServer.Routes(ctx))
	mux.Handle("/export/", exportHandler)

	// Federation
	federationInConfig := &federationin.Config{
		Timeout:        10 * time.Minute,
		TruncateWindow: 1 * time.Hour,
	}

	federationinHandler := federationin.NewHandler(env, federationInConfig)
	mux.Handle("/federation-in", federationinHandler)

	// Federation out
	// TODO: this is a grpc listener and requires a lot of setup.

	revConfig := revision.Config{
		KeyID:     tokenKey,
		AAD:       []byte{1, 2, 3},
		MinLength: 28,
	}

	// Key Rotation
	keyRotationConfig := &keyrotation.Config{
		RevisionToken: revConfig,

		// Very accellerated schedule for testing.
		NewKeyPeriod:       100 * time.Millisecond,
		DeleteOldKeyPeriod: 100 * time.Millisecond,
	}

	rotationServer, err := keyrotation.NewServer(keyRotationConfig, env)
	if err != nil {
		tb.Fatal(err)
	}
	mux.Handle("/key-rotation/", http.StripPrefix("/key-rotation", rotationServer.Routes(ctx)))

	// Parse the config to load default values.
	publishConfig := publish.Config{}
	envconfig.ProcessWith(ctx, &publishConfig, envconfig.OsLookuper())
	// Make overrides.
	publishConfig.RevisionToken = revConfig
	publishConfig.MaxKeysOnPublish = 15
	publishConfig.MaxSameStartIntervalKeys = 2
	publishConfig.MaxIntervalAge = 360 * time.Hour
	publishConfig.CreatedAtTruncateWindow = time.Second
	publishConfig.ReleaseSameDayKeys = true
	publishConfig.RevisionKeyCacheDuration = time.Second

	publishHandler, err := publish.NewHandler(ctx, &publishConfig, env)
	if err != nil {
		tb.Fatal(err)
	}
	mux.Handle("/publish", publishHandler.Handle())

	srv, err := server.New("")
	if err != nil {
		tb.Fatal(err)
	}

	// Stop the server on cleanup
	stopCtx, stop := context.WithCancel(ctx)
	tb.Cleanup(stop)

	// Start the server
	go func() {
		if err := srv.ServeHTTPHandler(stopCtx, mux); err != nil {
			tb.Error(err)
		}
	}()

	// Create a client
	client := testClient(tb, srv)

	enClient := &Client{client: client}

	return env, enClient
}

func Seed(tb testing.TB, ctx context.Context, db *database.DB, exportPeriod time.Duration) (*testutil.JWTConfig, string, string, string) {
	bucketName := testRandomID(tb, 32)
	filenameRoot := testRandomID(tb, 32)

	if v := os.Getenv("E2E_GOOGLE_CLOUD_BUCKET"); v != "" && !testing.Short() {
		bucketName = v
		tb.Logf("Bucket name is %q", bucketName)
	}

	// create a signing key
	sk := testutil.GetSigningKey(tb)

	// create a health authority
	ha := &vm.HealthAuthority{
		Audience: "exposure-notifications-service",
		Issuer:   fmt.Sprintf("Department of Health %s", testRandomID(tb, 32)[:6]),
		Name:     "Integration Test HA",
	}
	haKey := &vm.HealthAuthorityKey{
		Version: "v1",
		From:    time.Now().Add(-1 * time.Minute),
	}
	haID := testutil.InitalizeVerificationDB(ctx, tb, db, ha, haKey, sk)
	jwtCfg := &testutil.JWTConfig{
		HealthAuthority:    ha,
		HealthAuthorityKey: haKey,
		Key:                sk.Key,
		ReportType:         verifyapi.ReportTypeConfirmed,
	}

	appName := fmt.Sprintf("com.%s.app", testRandomID(tb, 32)[:6])
	// Create an authorized app
	authorizedapp := &authorizedappmodel.AuthorizedApp{
		AppPackageName: appName,
		AllowedRegions: map[string]struct{}{
			"TEST": {},
		},
		AllowedHealthAuthorityIDs: map[int64]struct{}{
			haID: {},
		},

		// TODO: hook up verification and revision
		BypassHealthAuthorityVerification: false,
		BypassRevisionToken:               false,
	}
	exist, err := authorizedappdb.New(db).GetAuthorizedApp(context.Background(), authorizedapp.AppPackageName)
	if exist == nil || err != nil {
		tb.Log("Creating a new authorized app")
		if err := authorizedappdb.New(db).InsertAuthorizedApp(context.Background(), &authorizedappmodel.AuthorizedApp{
			AppPackageName: appName,
			AllowedRegions: map[string]struct{}{
				"TEST": {},
			},
			AllowedHealthAuthorityIDs: map[int64]struct{}{
				haID: {},
			},

			BypassHealthAuthorityVerification: false,
			BypassRevisionToken:               false,
		}); err != nil {
			tb.Fatal(err)
		}
	}

	// Create a signature info
	si := &exportmodel.SignatureInfo{
		SigningKey:        "signer",
		SigningKeyVersion: "v1",
		SigningKeyID:      "US",
	}
	if err := exportdatabase.New(db).AddSignatureInfo(ctx, si); err != nil {
		tb.Fatal(err)
	}

	// Create an export config
	ec := &exportmodel.ExportConfig{
		BucketName:       bucketName,
		FilenameRoot:     filenameRoot,
		Period:           exportPeriod,
		OutputRegion:     "TEST",
		From:             time.Now().Add(-2 * time.Second),
		Thru:             time.Now().Add(1 * time.Hour),
		SignatureInfoIDs: []int64{},
	}
	if err := exportdatabase.New(db).AddExportConfig(ctx, ec); err != nil {
		tb.Fatal(err)
	}

	return jwtCfg, bucketName, filenameRoot, appName
}

type prefixRoundTripper struct {
	addr string
	rt   http.RoundTripper
}

func (p *prefixRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	u := r.URL
	if u.Scheme == "" {
		u.Scheme = "http"
	}
	if u.Host == "" {
		u.Host = p.addr
	}

	return p.rt.RoundTrip(r)
}

func testClient(tb testing.TB, srv *server.Server) *http.Client {
	prt := &prefixRoundTripper{
		addr: srv.Addr(),
		rt:   http.DefaultTransport,
	}

	return &http.Client{
		// Cleaning up 10000 multiple exports takes ~20 seconds, increasing this
		// so that load test doesn't time out
		Timeout:   50 * time.Second,
		Transport: prt,
	}
}

func testRandomID(tb testing.TB, size int) string {
	tb.Helper()

	b := make([]byte, size)
	if _, err := rand.Read(b[:]); err != nil {
		tb.Fatalf("failed to generate random: %v", err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(b[:]))
}

// Eventually retries the given function n times, sleeping 1s between each
// invocation. To mark an error as retryable, wrap it in retry.RetryableError.
// Non-retryable errors return immediately.
func Eventually(tb testing.TB, times uint64, interval time.Duration, f func() error) {
	ctx := context.Background()
	b, err := retry.NewConstant(interval)
	if err != nil {
		tb.Fatalf("failed to create retry: %v", err)
	}
	b = retry.WithMaxRetries(times, b)

	if err := retry.Do(ctx, b, func(ctx context.Context) error {
		return f()
	}); err != nil {
		tb.Fatalf("did not return ok after %d retries: %v", times, err)
	}
}

// IndexFilePath returns the filepath of index file under blob storage
func IndexFilePath(root string) string {
	return path.Join(root, "index.txt")
}
