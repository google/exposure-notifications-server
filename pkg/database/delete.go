package database

import (
	"cambio/pkg/logging"
	"context"
	"fmt"

	"cloud.google.com/go/datastore"
)

const batchSize = 500

// DeleteDiagnosisKeys deletes the keys provided from datastore. This is done
// in batches, and returns the number of records deleted in total, along with
// an error if any batch of deletes fail. If multiple batches fail, the last
// error is returned.
func DeleteDiagnosisKeys(ctx context.Context, keys []*datastore.Key) (int, error) {
	logger := logging.FromContext(ctx)

	client := Connection()
	if client == nil {
		return 0, fmt.Errorf("unable to obtain database client")
	}

	logger.Infof("Deleting %v records", len(keys))

	var err error
	count := 0
	for i := 0; i < len(keys); i += batchSize {
		j := i + batchSize
		if j > len(keys) {
			j = len(keys)
		}

		batch := keys[i:j]
		if err = client.DeleteMulti(ctx, batch); err != nil {
			logger.Errorf("DeleteMulti error between indices %v and %v", i, j)
		} else {
			count += len(batch)
		}
	}

	return count, err
}
