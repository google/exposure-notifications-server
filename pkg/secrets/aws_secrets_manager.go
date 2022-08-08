// Copyright 2020 the Exposure Notifications Server authors
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

//go:build aws || all

package secrets

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

func init() {
	RegisterManager("AWS_SECRETS_MANAGER", NewAWSSecretsManager)
}

// Compile-time check to verify implements interface.
var _ SecretManager = (*AWSSecretsManager)(nil)

// AWSSecretsManager implements SecretManager.
type AWSSecretsManager struct {
	svc *secretsmanager.SecretsManager
}

// NewAWSSecretsManager creates a new secret manager for AWS. Configuration is
// provided via the standard AWS environment variables.
func NewAWSSecretsManager(ctx context.Context, _ *Config) (SecretManager, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	svc := secretsmanager.New(sess)

	return &AWSSecretsManager{
		svc: svc,
	}, nil
}

// GetSecretValue implements the SecretManager interface. Secret names should be
// of the format:
//
//	SECRET@VERSION#STAGE
//
// Where:
//   - SECRET is the name or ARN of the secret
//   - VERSION is the version ID (default: "")
//   - Stage is the stage (one of AWSCURRENT or AWSPREVIOUS, default: "")
//
// Secrets are expected to be string plaintext values (not JSON, YAML,
// key-value, etc).
func (sm *AWSSecretsManager) GetSecretValue(ctx context.Context, name string) (string, error) {
	var secretID, versionID, versionStage string

	current := &secretID
	for _, ch := range name {
		if ch == '@' {
			current = &versionID
			continue
		}

		if ch == '#' {
			current = &versionStage
			continue
		}

		*current += string(ch)
	}

	req := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretID),
	}

	if versionID != "" {
		req.VersionId = aws.String(versionID)
	}

	if versionStage != "" {
		req.VersionStage = aws.String(versionStage)
	}

	result, err := sm.svc.GetSecretValueWithContext(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to access secret %v: %w", name, err)
	}

	if v := aws.StringValue(result.SecretString); v != "" {
		return v, nil
	}

	return string(result.SecretBinary), nil
}
