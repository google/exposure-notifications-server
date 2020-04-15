package database

import (
	"cambio/pkg/logging"
	"context"
	"fmt"
)

const batchSize = 500

// DeleteDiagnosisKeys deletes the keys provided from datastore. This is done
// in batches, and returns an error if any batch of deletes fail. If multiple
// batches fail, the last error is returned.
func DeleteDiagnosisKeys(ctx context.Context, keys []string) error {
	logger := logging.FromContext(ctx)

	client := Connection()
	if client == nil {
		return fmt.Errorf("unable to obtain database client")
	}

	logger.Infof("Deleting %v records", len(keys))

	var err error
	for i := 0; i < len(keys); i += batchSize {
		j := i + batchSize
		if j > len(keys) {
			j = len(keys)
		}

		if err = client.DeleteMulti(ctx, keys[i:j]); err != nil {
			logger.Errorf("DeleteMulti error between indices %v and %v", i, j)
		}
	}

	return err
}
