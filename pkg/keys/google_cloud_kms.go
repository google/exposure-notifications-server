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

	kms "cloud.google.com/go/kms/apiv1"
	"github.com/sethvargo/go-gcpkms/pkg/gcpkms"
	kmspb "google.golang.org/genproto/googleapis/cloud/kms/v1"
)

// Compile-time check to verify implements interface.
var _ KeyManager = (*GoogleCloudKMS)(nil)

// GoogleCloudKMS implements the keys.KeyManager interface and can be used to sign
// export files.
type GoogleCloudKMS struct {
	client *kms.KeyManagementClient
}

func NewGoogleCloudKMS(ctx context.Context) (KeyManager, error) {
	client, err := kms.NewKeyManagementClient(ctx)
	if err != nil {
		return nil, err
	}
	return &GoogleCloudKMS{client}, nil
}

func (kms *GoogleCloudKMS) NewSigner(ctx context.Context, keyID string) (crypto.Signer, error) {
	return gcpkms.NewSigner(ctx, kms.client, keyID)
}

func (kms *GoogleCloudKMS) Encrypt(ctx context.Context, keyID string, plaintext []byte, aad string) ([]byte, error) {
	req := kmspb.EncryptRequest{
		Name:                        keyID,
		Plaintext:                   plaintext,
		AdditionalAuthenticatedData: []byte(aad),
	}
	result, err := kms.client.Encrypt(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt: %w", err)
	}
	return result.Ciphertext, nil
}

func (kms *GoogleCloudKMS) Decrypt(ctx context.Context, keyID string, ciphertext []byte, aad string) ([]byte, error) {
	req := kmspb.DecryptRequest{
		Name:                        keyID,
		Ciphertext:                  ciphertext,
		AdditionalAuthenticatedData: []byte(aad),
	}
	result, err := kms.client.Decrypt(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}
	return result.Plaintext, nil
}
