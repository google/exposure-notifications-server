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

// Package main provides a utility that bootstraps the initial database with
// users and realms.
package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	authorizedappdatabase "github.com/google/exposure-notifications-server/internal/authorizedapp/database"
	authorizedappmodel "github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	"github.com/google/exposure-notifications-server/internal/buildinfo"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/setup"
	verificationdatabase "github.com/google/exposure-notifications-server/internal/verification/database"
	verificationmodel "github.com/google/exposure-notifications-server/internal/verification/model"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/sethvargo/go-signalcontext"
)

func main() {
	ctx, done := signalcontext.OnInterrupt()

	logger := logging.NewLoggerFromEnv().Named("tools.seed").
		With("build_id", buildinfo.KeyServer.ID()).
		With("build_tag", buildinfo.KeyServer.Tag())
	ctx = logging.WithLogger(ctx, logger)

	err := realMain(ctx)
	done()

	if err != nil {
		logger.Fatal(err)
	}
}

func realMain(ctx context.Context) error {
	var config database.Config
	env, err := setup.Setup(ctx, &config)
	if err != nil {
		return fmt.Errorf("failed to setup database: %w", err)
	}
	defer env.Close(ctx)

	db := env.Database()
	aadb := authorizedappdatabase.New(db)
	verifydb := verificationdatabase.New(db)

	// Create the revision token encrypter.
	if err := createEncryptionKey(ctx, "revision-token-encrypter"); err != nil {
		return err
	}

	// Create a health authority.
	var ha verificationmodel.HealthAuthority
	ha.ID = 1
	ha.Issuer = "iss-test"
	ha.Audience = "aud-test"
	ha.Name = "Health Systems, Inc."
	if err := verifydb.AddHealthAuthority(ctx, &ha); err != nil {
		return fmt.Errorf("failed to create health authority: %w", err)
	}

	// Create a health authority key.
	publicKeyPEM, err := createSigningKey(ctx, "health-authority-1")
	if err != nil {
		return err
	}

	var hak verificationmodel.HealthAuthorityKey
	hak.AuthorityID = 1
	hak.Version = "1"
	hak.From = time.Now()
	hak.Thru = time.Now().Add(365 * 24 * time.Hour)
	hak.PublicKeyPEM = publicKeyPEM
	if err := verifydb.AddHealthAuthorityKey(ctx, &ha, &hak); err != nil {
		return fmt.Errorf("failed to add health authority key: %w", err)
	}

	// Create some authorized apps.
	iosApp := authorizedappmodel.NewAuthorizedApp()
	iosApp.AppPackageName = "com.example.ios.app"
	iosApp.AllowedRegions = map[string]struct{}{"US": {}}
	iosApp.AllowedHealthAuthorityIDs = map[int64]struct{}{ha.ID: {}}
	if err := aadb.InsertAuthorizedApp(ctx, iosApp); err != nil {
		return fmt.Errorf("failed to create ios app: %w", err)
	}

	androidApp := authorizedappmodel.NewAuthorizedApp()
	androidApp.AppPackageName = "com.example.android.app"
	androidApp.AllowedRegions = map[string]struct{}{"US": {}}
	androidApp.AllowedHealthAuthorityIDs = map[int64]struct{}{ha.ID: {}}
	if err := aadb.InsertAuthorizedApp(ctx, androidApp); err != nil {
		return fmt.Errorf("failed to create android app: %w", err)
	}

	return nil
}

func createEncryptionKey(ctx context.Context, name string) error {
	_, self, _, ok := runtime.Caller(1)
	if !ok {
		return fmt.Errorf("failed to get caller")
	}

	localDir := filepath.Join(filepath.Dir(self), "../../local/keys")
	kms, err := keys.NewFilesystem(ctx, localDir)
	if err != nil {
		return err
	}

	parent, err := kms.CreateEncryptionKey(ctx, "system", name)
	if err != nil {
		return err
	}
	if _, err := kms.CreateKeyVersion(ctx, parent); err != nil {
		return err
	}

	return nil
}

func createSigningKey(ctx context.Context, name string) (string, error) {
	_, self, _, ok := runtime.Caller(1)
	if !ok {
		return "", fmt.Errorf("failed to get caller")
	}

	localDir := filepath.Join(filepath.Dir(self), "../../local/keys")
	kms, err := keys.NewFilesystem(ctx, localDir)
	if err != nil {
		return "", err
	}

	parent, err := kms.CreateSigningKey(ctx, "system", name)
	if err != nil {
		return "", err
	}
	list, err := kms.SigningKeyVersions(ctx, parent)
	if err != nil {
		return "", err
	}

	if len(list) == 0 {
		if _, err := kms.CreateKeyVersion(ctx, parent); err != nil {
			return "", err
		}

		list, err = kms.SigningKeyVersions(ctx, parent)
		if err != nil {
			return "", err
		}
	}

	signer, err := list[0].Signer(ctx)
	if err != nil {
		return "", err
	}

	publicKey := signer.Public()
	pemBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}

	var b bytes.Buffer
	if err := pem.Encode(&b, &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pemBytes,
	}); err != nil {
		return "", err
	}

	return b.String(), nil
}
