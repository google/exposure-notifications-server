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

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/lstoll/awskms"
)

// Compile-time check to verify implements interface.
var _ KeyManager = (*AWSKMS)(nil)

// AWSKMS implements the keys.KeyManager interface and can be used to sign
// export files using AWS KMS.
type AWSKMS struct {
	svc *kms.KMS
}

func NewAWSKMS(ctx context.Context) (KeyManager, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	svc := kms.New(sess)

	return &AWSKMS{
		svc: svc,
	}, nil
}

func (s *AWSKMS) NewSigner(ctx context.Context, keyID string) (crypto.Signer, error) {
	return awskms.NewSigner(ctx, s.svc, keyID)
}

func buildAAD(aad string) map[string]*string {
	m := make(map[string]*string)
	m["aad"] = &aad
	return m
}

func (s *AWSKMS) Encrypt(ctx context.Context, keyID string, plaintext []byte, aad string) ([]byte, error) {
	input := kms.EncryptInput{
		KeyId:             &keyID,
		EncryptionContext: buildAAD(aad),
		Plaintext:         plaintext,
	}
	output, err := s.svc.EncryptWithContext(ctx, &input)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt: %w", err)
	}
	return output.CiphertextBlob, nil
}

func (s *AWSKMS) Decrypt(ctx context.Context, keyID string, ciphertext []byte, aad string) ([]byte, error) {
	input := kms.DecryptInput{
		KeyId:             &keyID,
		EncryptionContext: buildAAD(aad),
		CiphertextBlob:    ciphertext,
	}
	output, err := s.svc.DecryptWithContext(ctx, &input)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}
	return output.Plaintext, nil
}
