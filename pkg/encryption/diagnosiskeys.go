package encryption

import (
	"cambio/pkg/model"
	"context"
	"fmt"
	"os"

	kms "cloud.google.com/go/kms/apiv1"
	kmspb "google.golang.org/genproto/googleapis/cloud/kms/v1"
)

var (
	keyName string
	client  *kms.KeyManagementClient
)

func InitDiagnosisKeys() error {
	// Google Cloud KMS, Key format:
	// projects/[PROJECT]/locations/[LOCATION]/keyRings/[KEYRING]/cryptoKeys/[KEY]
	// See the KMS object hierarchy here: https://cloud.google.com/kms/docs/object-hierarchy
	keyName = os.Getenv("DIAGNOSIS_KMS_KEY")
	if keyName == "" {
		return fmt.Errorf("missing required environment variable, `DIAGNOSIS_KMS_KEY`")
	}

	if client != nil {
		return nil
	}

	ctx := context.Background()
	var err error
	client, err = kms.NewKeyManagementClient(ctx)
	if err != nil {
		return fmt.Errorf("creating KMS client: %v", err)
	}
	return nil
}

// EncryptDiagnosisKeys encrypts the diagnosis keys with the configured
// key from Google Cloud KMS.
// the infections slice passed in is modified in-place making the plaintext
// DiagnosisKey fields inaccessible in memory after the encrypt operation.
func EncryptDiagnosisKeys(ctx context.Context, infections []model.Infection) error {
	for i, infection := range infections {
		req := &kmspb.EncryptRequest{
			Name:      keyName,
			Plaintext: infection.DiagnosisKey,
		}
		result, err := client.Encrypt(ctx, req)
		if err != nil {
			return fmt.Errorf("encrypting diagnosis key: %v", err)
		}
		infections[i].DiagnosisKey = result.Ciphertext
	}
	return nil
}
