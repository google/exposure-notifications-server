package database

import (
	"cambio/pkg/logging"
	"context"
	"fmt"
	"time"
)

func FilterKeysOnly(ctx context.Context, entity string, to time.Time) ([]string, error) {
	logger := logging.FromContext(ctx)

	client := Connection()
	if client == nil {
		return nil, fmt.Errorf("unable to obtain database client")
	}

	q := client.NewQuery(entity).Filter("createdAt <=", to).KeysOnly()

	var keys []string
	// TODO: do this using cursors
	_, err := client.GetAll(ctx, q, &keys)
	if err != nil {
		return nil, err
	}

	logger.Infof("Returning %v keys", len(keys))
	return keys, nil
}
