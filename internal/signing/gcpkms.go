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

package signing

import (
	"context"
	"crypto"

	kms "cloud.google.com/go/kms/apiv1"
	"github.com/sethvargo/go-gcpkms/pkg/gcpkms"
)

// Compile-time check to verify implements interface.
var _ KeyManager = (*GCPKMS)(nil)

// GCPKMS implements the signing.KeyManager interface and can be used to sign
// export files.
type GCPKMS struct {
	client *kms.KeyManagementClient
}

func NewGCPKMS(ctx context.Context) (KeyManager, error) {
	client, err := kms.NewKeyManagementClient(ctx)
	if err != nil {
		return nil, err
	}
	return &GCPKMS{client}, nil
}

func (kms *GCPKMS) NewSigner(ctx context.Context, keyID string) (crypto.Signer, error) {
	signer, err := gcpkms.NewSigner(ctx, kms.client, keyID)
	if err != nil {
		return nil, err
	}
	return signer, nil
}
