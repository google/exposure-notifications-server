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
	"fmt"
	"time"

	kms "cloud.google.com/go/kms/apiv1"
	"github.com/sethvargo/go-gcpkms/pkg/gcpkms"
	"google.golang.org/api/iterator"
	kmspb "google.golang.org/genproto/googleapis/cloud/kms/v1"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// Compile-time check to verify implements interface.
var _ KeyManager = (*GoogleCloudKMS)(nil)
var _ SigningKeyManager = (*GoogleCloudKMS)(nil)

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

func (k *CloudKMSSigningKeyVersion) DestroyedAt() time.Time {
	return k.destroyedAt
}

func (k *CloudKMSSigningKeyVersion) Signer(ctx context.Context) (crypto.Signer, error) {
	return k.keyManager.NewSigner(ctx, k.keyID)
}

func NewGoogleCloudKMS(ctx context.Context, config *Config) (KeyManager, error) {
	client, err := kms.NewKeyManagementClient(ctx)
	if err != nil {
		return nil, err
	}
	return &GoogleCloudKMS{client, config.CreateHSMKeys}, nil
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

// CreateSigningKey creates a new signing key in Cloud KMS. If a key already
// exists, it returns the existing key.
func (kms *GoogleCloudKMS) CreateSigningKey(ctx context.Context, parent, name string) (string, error) {
	result, err := kms.client.CreateCryptoKey(ctx, &kmspb.CreateCryptoKeyRequest{
		Parent:      parent,
		CryptoKeyId: name,
		CryptoKey: &kmspb.CryptoKey{
			Purpose: kmspb.CryptoKey_ASYMMETRIC_SIGN,
			VersionTemplate: &kmspb.CryptoKeyVersionTemplate{
				ProtectionLevel: kms.protectionLevel(),
				Algorithm:       kmspb.CryptoKeyVersion_EC_SIGN_P256_SHA256,
			},
		},
	})
	if err != nil {
		if grpcstatus.Code(err) == grpccodes.AlreadyExists {
			// The key already exists, just return it.
			return fmt.Sprintf("%s/cryptoKeys/%s", parent, name), nil
		}

		return "", fmt.Errorf("failed to create signing key: %w", err)
	}
	return result.Name, nil
}

// SigningKeyVersions returns the list of key versions for the parent parsed as
// signing keys.
func (kms *GoogleCloudKMS) SigningKeyVersions(ctx context.Context, parent string) ([]SigningKeyVersion, error) {
	results := make([]SigningKeyVersion, 0, 32)

	it := kms.client.ListCryptoKeyVersions(ctx, &kmspb.ListCryptoKeyVersionsRequest{
		Parent:   parent,
		PageSize: 200,
		Filter:   `Filter: "state != DESTROYED AND state != DESTROY_SCHEDULED"`,
	})
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list keys: %w", err)
		}

		var key CloudKMSSigningKeyVersion
		key.keyID = resp.Name
		key.keyManager = kms

		if t := resp.GetCreateTime(); t != nil {
			key.createdAt = t.AsTime()
		}
		if t := resp.GetDestroyEventTime(); t != nil {
			key.destroyedAt = t.AsTime()
		}

		results = append(results, &key)
	}

	return results, nil
}

// CreateKeyVersion creates a new version for the given key. The parent key must
// already exist.
func (kms *GoogleCloudKMS) CreateKeyVersion(ctx context.Context, parent string) (string, error) {
	result, err := kms.client.CreateCryptoKeyVersion(ctx, &kmspb.CreateCryptoKeyVersionRequest{
		Parent: parent,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create key version: %w", err)
	}
	return result.Name, nil
}

// DestroyKeyVersion marks the given key version for destruction. If the version
// does not exist, it does nothing. The id is the full resource name like
// projects/locations/keyRings/...
func (kms *GoogleCloudKMS) DestroyKeyVersion(ctx context.Context, id string) error {
	if _, err := kms.client.DestroyCryptoKeyVersion(ctx, &kmspb.DestroyCryptoKeyVersionRequest{
		Name: id,
	}); err != nil {
		// Cloud KMS returns the following errors when the key is already destroyed
		// or does not exist.
		code := grpcstatus.Code(err)
		if code == grpccodes.NotFound || code == grpccodes.FailedPrecondition {
			return nil
		}

		return fmt.Errorf("failed to destroy key version: %w", err)
	}

	return nil
}

func (kms *GoogleCloudKMS) protectionLevel() kmspb.ProtectionLevel {
	if kms.useHSM {
		return kmspb.ProtectionLevel_HSM
	}
	return kmspb.ProtectionLevel_SOFTWARE
}
