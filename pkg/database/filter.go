package database

import (
	"cambio/pkg/logging"
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/datastore"
)

func FilterKeysOnly(ctx context.Context, to time.Time) ([]*datastore.Key, error) {
	logger := logging.FromContext(ctx)

	client := Connection()
	if client == nil {
		return nil, fmt.Errorf("unable to obtain database client")
	}

	q := datastore.NewQuery("infection").Filter("createdAt <=", to).KeysOnly()

	var keys []*datastore.Key
	// TODO: do this using cursors
	_, err := client.GetAll(ctx, q, &keys)
	if err != nil {
		return nil, err
	}

	logger.Infof("Returning %v keys", len(keys))
	return keys, nil
}
