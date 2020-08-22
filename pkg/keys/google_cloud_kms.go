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

package keys

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"time"

	kms "cloud.google.com/go/kms/apiv1"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/sethvargo/go-gcpkms/pkg/gcpkms"
	"google.golang.org/api/iterator"
	kmspb "google.golang.org/genproto/googleapis/cloud/kms/v1"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// Compile-time check to verify implements interface.
var _ KeyManager = (*GoogleCloudKMS)(nil)
var _ SigningKeyManagement = (*GoogleCloudKMS)(nil)

var (
	ErrKeyExists = errors.New("key already exists")
)

// GoogleCloudKMS implements the keys.KeyManager interface and can be used to sign
// export files.
type GoogleCloudKMS struct {
	client *kms.KeyManagementClient
	useHSM bool
}

type CloudKMSSigningKeyVersion struct {
	keyID       string
	createdAt   time.Time
	destroyedAt time.Time
	keyManager  *GoogleCloudKMS
}

func (k *CloudKMSSigningKeyVersion) KeyID() string {
	return k.keyID
}

func (k *CloudKMSSigningKeyVersion) CreatedAt() time.Time {
	return k.createdAt
}

func (k *CloudKMSSigningKeyVersion) DetroyedAt() time.Time {
	return k.destroyedAt
}

func (k *CloudKMSSigningKeyVersion) GetSigner(ctx context.Context) (crypto.Signer, error) {
	return k.keyManager.NewSigner(ctx, k.keyID)
}

func NewGoogleCloudKMS(ctx context.Context, useHSM bool) (KeyManager, error) {
	client, err := kms.NewKeyManagementClient(ctx)
	if err != nil {
		return nil, err
	}
	return &GoogleCloudKMS{client, useHSM}, nil
}

func (kms *GoogleCloudKMS) NewSigner(ctx context.Context, keyID string) (crypto.Signer, error) {
	return gcpkms.NewSigner(ctx, kms.client, keyID)
}

func (kms *GoogleCloudKMS) Encrypt(ctx context.Context, keyID string, plaintext []byte, aad []byte) ([]byte, error) {
	req := kmspb.EncryptRequest{
		Name:                        keyID,
		Plaintext:                   plaintext,
		AdditionalAuthenticatedData: aad,
	}
	result, err := kms.client.Encrypt(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt: %w", err)
	}
	return result.Ciphertext, nil
}

func (kms *GoogleCloudKMS) Decrypt(ctx context.Context, keyID string, ciphertext []byte, aad []byte) ([]byte, error) {
	req := kmspb.DecryptRequest{
		Name:                        keyID,
		Ciphertext:                  ciphertext,
		AdditionalAuthenticatedData: aad,
	}
	result, err := kms.client.Decrypt(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}
	return result.Plaintext, nil
}

func (kms *GoogleCloudKMS) CreateSigningKeyVersion(ctx context.Context, keyRing string, name string) (string, error) {
	logger := logging.FromContext(ctx)
	getRequest := kmspb.GetCryptoKeyRequest{
		Name: fmt.Sprintf("%s/cryptoKeys/%s", keyRing, name),
	}
	logger.Infow("gcpkms.GetCryptoKey", "keyring", keyRing, "name", name)
	key, err := kms.client.GetCryptoKey(ctx, &getRequest)
	if err != nil {
		return "", fmt.Errorf("cannot create version, key does not exist: %w", err)
	}

	createRequest := kmspb.CreateCryptoKeyVersionRequest{
		Parent: key.Name,
	}
	ver, err := kms.client.CreateCryptoKeyVersion(ctx, &createRequest)
	if err != nil {
		return "", fmt.Errorf("gcpkms.CreateCryptoKeyVersion: %w", err)
	}
	return ver.Name, nil
}

func (kms *GoogleCloudKMS) GetSigningKeyVersions(ctx context.Context, keyRing string, name string) ([]SigningKeyVersion, error) {
	_, err := kms.getOrCreateSigningKey(ctx, keyRing, name)
	if err != nil {
		return nil, fmt.Errorf("unable to get crypto key: %w", err)
	}

	request := kmspb.ListCryptoKeyVersionsRequest{
		Parent:   fmt.Sprintf("%s/cryptoKeys/%s", keyRing, name),
		PageSize: 200,
		Filter:   `Filter: "state != DESTROYED AND state != DESTROY_SCHEDULED"`,
	}

	results := make([]SigningKeyVersion, 0)

	it := kms.client.ListCryptoKeyVersions(ctx, &request)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error listing crypto keys: %w", err)
		}

		key := CloudKMSSigningKeyVersion{
			keyID:      resp.Name,
			createdAt:  resp.GetCreateTime().AsTime(),
			keyManager: kms,
		}
		if resp.DestroyEventTime != nil {
			key.destroyedAt = resp.GetDestroyEventTime().AsTime()
		}
		results = append(results, &key)
	}

	return results, nil
}

func (kms *GoogleCloudKMS) ProtectionLevel() kmspb.ProtectionLevel {
	if kms.useHSM {
		return kmspb.ProtectionLevel_HSM
	}
	return kmspb.ProtectionLevel_SOFTWARE
}

func (kms *GoogleCloudKMS) getOrCreateSigningKey(ctx context.Context, keyRing string, name string) (*kmspb.CryptoKey, error) {
	logger := logging.FromContext(ctx)
	getRequest := kmspb.GetCryptoKeyRequest{
		Name: fmt.Sprintf("%s/cryptoKeys/%s", keyRing, name),
	}
	logger.Infow("gcpkms.GetCryptoKey", "keyring", keyRing, "name", name)
	key, err := kms.client.GetCryptoKey(ctx, &getRequest)
	if err == nil {
		return key, nil
	}

	// Attempt to create the crypto key in this key ring w/ our default settings.
	createRequest := kmspb.CreateCryptoKeyRequest{
		Parent:      keyRing,
		CryptoKeyId: name,
		CryptoKey: &kmspb.CryptoKey{
			Purpose: kmspb.CryptoKey_ASYMMETRIC_SIGN,
			VersionTemplate: &kmspb.CryptoKeyVersionTemplate{
				ProtectionLevel: kms.ProtectionLevel(),
				Algorithm:       kmspb.CryptoKeyVersion_EC_SIGN_P256_SHA256,
			},
		},
	}
	key, err = kms.client.CreateCryptoKey(ctx, &createRequest)
	if err != nil {
		if terr, ok := grpcstatus.FromError(err); ok && terr.Code() == grpccodes.AlreadyExists {
			return nil, ErrKeyExists
		}
		return nil, fmt.Errorf("unable to create signing key: %w", err)
	}
	return key, nil
}
