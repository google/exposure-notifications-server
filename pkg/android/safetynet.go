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

// package android managed device attestation inegation with Android's
// SafetyNet API.
package android

import (
	"context"
	"fmt"
	"os"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

var (
	apiKey = ""
	client *secretmanager.Client
)

func InitializeSafetynet() error {
	if apiKey != "" {
		return nil
	}

	ctx := context.Background()
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("secretmanager.NewClient: %v", err)
	}

	secretName := os.Getenv("SAFETYNET_API_KEY")
	if secretName == "" {
		return fmt.Errorf("missing environment variable SAFETYNET_API_KEY")
	}

	// Build the request.
	accessRequest := &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretName,
	}

	// Call the API.
	result, err := client.AccessSecretVersion(ctx, accessRequest)
	if err != nil {
		return fmt.Errorf("failed to access secret version: %v", err)
	}

	apiKey = string(result.Payload.Data)

	return nil
}

func VerifySafetynet(payload, appPackageName string, diagnosisKeys []string, regions []string) {

}
