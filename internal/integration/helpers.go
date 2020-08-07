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
	vdb "github.com/google/exposure-notifications-server/internal/verification/database"
	vm "github.com/google/exposure-notifications-server/internal/verification/model"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/google/exposure-notifications-server/pkg/server"
	"github.com/sethvargo/go-retry"

	authorizedappmodel "github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	exportdatabase "github.com/google/exposure-notifications-server/internal/export/database"
	exportmodel "github.com/google/exposure-notifications-server/internal/export/model"
	testutil "github.com/google/exposure-notifications-server/internal/utils"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
)

var (
	ExportDir    = "my-bucket"
	FileNameRoot = "/"
)

// NewTestServer sets up clients used for integration tests
func NewTestServer(tb testing.TB, exportPeriod time.Duration) (*serverenv.ServerEnv, *Client, *database.DB, testutil.JWTConfig) {
	ctx := context.Background()
	env, client, jwtCfg := testServer(tb)
	db := env.Database()
	enClient := &Client{client: client}

	// Create an authorized app
	aa := env.AuthorizedAppProvider()
	if err := aa.Add(ctx, &authorizedappmodel.AuthorizedApp{
		AppPackageName: "com.example.app",
		AllowedRegions: map[string]struct{}{
			"TEST": {},
		},
		AllowedHealthAuthorityIDs: map[int64]struct{}{
			1: {},
		},

		// TODO: hook up verification, and disable
		BypassHealthAuthorityVerification: false,
	}); err != nil {
		tb.Fatal(err)
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
		BucketName:       ExportDir,
		FilenameRoot:     FileNameRoot,
		Period:           exportPeriod,
		OutputRegion:     "TEST",
		From:             time.Now().Add(-2 * time.Second),
		Thru:             time.Now().Add(1 * time.Hour),
		SignatureInfoIDs: []int64{},
	}
	if err := exportdatabase.New(db).AddExportConfig(ctx, ec); err != nil {
		tb.Fatal(err)
	}

	return env, enClient, db, jwtCfg
}

// testServer sets up mocked local servers for running tests
func testServer(tb testing.TB) (*serverenv.ServerEnv, *http.Client, testutil.JWTConfig) {
	tb.Helper()

	var (
		ctx    = context.Background()
		jwtCfg = testutil.JWTConfig{}
	)

	aa, err := authorizedapp.NewMemoryProvider(ctx, nil)
	if err != nil {
		tb.Fatal(err)
	}

	if FileNameRoot, err = randomString(); err != nil {
		tb.Fatal(err)
	}
	bs, err := storage.NewMemory(ctx)
	if v := os.Getenv("GOOGLE_CLOUD_BUCKET"); v != "" && !testing.Short() {
		ExportDir = os.Getenv("GOOGLE_CLOUD_BUCKET")
		bs, err = storage.NewGoogleCloudStorage(ctx)
	}
	if err != nil {
		tb.Fatal(err)
	}

	db := database.NewTestDatabase(tb)

	km, err := keys.NewInMemory(ctx)
	if err != nil {
		tb.Fatal(err)
	}
	if _, err := km.CreateEncryptionKey("tokenkey"); err != nil {
		tb.Fatal(err)
	}
	if _, err := km.CreateSigningKey("signingkey"); err != nil {
		tb.Fatal(err)
	}
	// create an initial revision key.
	revisionDB, err := revdb.New(db, &revdb.KMSConfig{WrapperKeyID: "tokenkey", KeyManager: km})
	if err != nil {
		tb.Fatal(err)
	}
	if _, err := revisionDB.CreateRevisionKey(ctx); err != nil {
		tb.Fatal(err)
	}

	verifyDB := vdb.New(db)

	// create a signing key
	sk := testutil.GetSigningKey(tb)

	// create a health authority
	ha := &vm.HealthAuthority{
		Audience: "exposure-notifications-service",
		Issuer:   "Department of Health",
		Name:     "Integration Test HA",
	}
	haKey := &vm.HealthAuthorityKey{
		Version: "v1",
		From:    time.Now().Add(-1 * time.Minute),
	}
	haKey.PublicKeyPEM = sk.PublicKey
	verifyDB.AddHealthAuthority(ctx, ha)
	verifyDB.AddHealthAuthorityKey(ctx, ha, haKey)

	// jwt config to be used to get a verification certificate
	jwtCfg = testutil.JWTConfig{
		HealthAuthority:    ha,
		HealthAuthorityKey: haKey,
		Key:                sk.Key,
		JWTWarp:            time.Duration(0),
		ReportType:         verifyapi.ReportTypeConfirmed,
	}

	sm, err := secrets.NewInMemory(ctx)
	if err != nil {
		tb.Fatal(err)
	}

	env := serverenv.New(ctx,
		serverenv.WithAuthorizedAppProvider(aa),
		serverenv.WithBlobStorage(bs),
		serverenv.WithDatabase(db),
		serverenv.WithKeyManager(km),
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
	mux.Handle("/export/", http.StripPrefix("/export", exportServer.Routes(ctx)))

	// Federation
	federationInConfig := &federationin.Config{
		Timeout:        10 * time.Minute,
		TruncateWindow: 1 * time.Hour,
	}

	mux.Handle("/federation-in", federationin.NewHandler(env, federationInConfig))

	// Federation out
	// TODO: this is a grpc listener and requires a lot of setup.

	revConfig := revision.Config{
		KeyID:     "tokenkey",
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

	// Publish
	publishConfig := &publish.Config{
		RevisionToken:            revConfig,
		MaxKeysOnPublish:         15,
		MaxSameStartIntervalKeys: 2,
		MaxIntervalAge:           360 * time.Hour,
		CreatedAtTruncateWindow:  1 * time.Second,
		ReleaseSameDayKeys:       true,
		RevisionKeyCacheDuration: time.Second,
	}

	publishHandler, err := publish.NewHandler(ctx, publishConfig, env)
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

	return env, client, jwtCfg
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

func randomString() (string, error) {
	var b [512]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("failed to generate random: %w", err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(b[:]))
	return digest[:32], nil
}

// Eventually retries the given function n times, sleeping 1s between each
// invocation. To mark an error as retryable, wrap it in retry.RetryableError.
// Non-retryable errors return immediately.
func Eventually(tb testing.TB, times uint64, f func() error) {
	ctx := context.Background()
	b, err := retry.NewConstant(1 * time.Second)
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

func IndexFilePath() string {
	return path.Join(FileNameRoot, "index.txt")
}
