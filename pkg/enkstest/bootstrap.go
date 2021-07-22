// Copyright 2021 the Exposure Notifications Server authors
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
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	authorizedappdatabase "github.com/google/exposure-notifications-server/internal/authorizedapp/database"
	authorizedappmodel "github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	exportdatabase "github.com/google/exposure-notifications-server/internal/export/database"
	exportmodel "github.com/google/exposure-notifications-server/internal/export/model"
	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	verificationdatabase "github.com/google/exposure-notifications-server/internal/verification/database"
	verificationmodel "github.com/google/exposure-notifications-server/internal/verification/model"
	"github.com/google/exposure-notifications-server/pkg/keys"
)

const (
	Audience = "test-aud"
	Issuer   = "test-iss"
	Name     = "Test issuer"
)

// BootstrapResponse is the response from a successful bootstrap.
type BootstrapResponse struct {
	Signer             crypto.Signer
	HealthAuthority    *verificationmodel.HealthAuthority
	HealthAuthorityKey *verificationmodel.HealthAuthorityKey
	AuthorizedApp      *authorizedappmodel.AuthorizedApp
	SignatureInfo      *exportmodel.SignatureInfo
	ExportConfig       *exportmodel.ExportConfig
}

// Bootstrap configures the database with default data.
func Bootstrap(ctx context.Context, env *serverenv.ServerEnv) (*BootstrapResponse, error) {
	db := env.Database()

	// Generate a random name
	randomName, err := project.RandomHexString(6)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random name: %w", err)
	}

	// Get the key manager
	keyManager := env.KeyManager()
	signingKeyManager, ok := keyManager.(keys.SigningKeyManager)
	if !ok {
		return nil, fmt.Errorf("key manager is not SigningKeyManager %T", keyManager)
	}

	// Create signing key for the health authority
	signingKey, err := signingKeyManager.CreateSigningKey(ctx, "bootstrap", "ha-signer")
	if err != nil {
		return nil, fmt.Errorf("failed to create signing key: %w", err)
	}
	signingKeyVersion, err := signingKeyManager.CreateKeyVersion(ctx, signingKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create signing key version: %w", err)
	}
	signingKeySigner, err := keyManager.NewSigner(ctx, signingKeyVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to create signing key signer: %w", err)
	}
	signingKeyPublicEncoded, err := x509.MarshalPKIXPublicKey(signingKeySigner.Public())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}
	signingKeyPublicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: signingKeyPublicEncoded})

	// Create health authority
	healthAuthority := &verificationmodel.HealthAuthority{
		Audience: Audience,
		Issuer:   Issuer,
		Name:     Name,
	}
	if err := verificationdatabase.New(db).AddHealthAuthority(ctx, healthAuthority); err != nil {
		return nil, fmt.Errorf("failed to add health authority: %w", err)
	}

	// Create health authority key
	healthAuthorityKey := &verificationmodel.HealthAuthorityKey{
		Version:      "v1",
		From:         time.Now().UTC().Add(-1 * time.Hour),
		PublicKeyPEM: string(signingKeyPublicPEM),
	}
	if err := verificationdatabase.New(db).AddHealthAuthorityKey(ctx, healthAuthority, healthAuthorityKey); err != nil {
		return nil, fmt.Errorf("failed to add health authority key: %w", err)
	}

	// Create authorized app
	authorizedApp := &authorizedappmodel.AuthorizedApp{
		AppPackageName: "com.test.app",
		AllowedRegions: map[string]struct{}{
			"TEST": {},
		},
		AllowedHealthAuthorityIDs: map[int64]struct{}{
			healthAuthority.ID: {},
		},
	}
	if err := authorizedappdatabase.New(db).InsertAuthorizedApp(ctx, authorizedApp); err != nil {
		return nil, fmt.Errorf("failed to insert authorized app: %w", err)
	}

	// Create signature info
	signatureInfo := &exportmodel.SignatureInfo{
		SigningKey:        signingKeyVersion,
		SigningKeyVersion: "v1",
		SigningKeyID:      "TEST",
	}
	if err := exportdatabase.New(db).AddSignatureInfo(ctx, signatureInfo); err != nil {
		return nil, fmt.Errorf("failed to add signature info: %w", err)
	}

	// Create export config
	exportConfig := &exportmodel.ExportConfig{
		BucketName:       fmt.Sprintf("bucket-%s", randomName),
		FilenameRoot:     fmt.Sprintf("root-%s", randomName),
		Period:           1 * time.Second,
		OutputRegion:     "TEST",
		From:             time.Now().UTC().Add(-1 * time.Hour),
		Thru:             time.Now().UTC().Add(1 * time.Hour),
		SignatureInfoIDs: []int64{signatureInfo.ID},
	}
	if err := exportdatabase.New(db).AddExportConfig(ctx, exportConfig); err != nil {
		return nil, fmt.Errorf("failed to add export config: %w", err)
	}

	return &BootstrapResponse{
		Signer:             signingKeySigner,
		HealthAuthority:    healthAuthority,
		HealthAuthorityKey: healthAuthorityKey,
		AuthorizedApp:      authorizedApp,
		SignatureInfo:      signatureInfo,
		ExportConfig:       exportConfig,
	}, nil
}
