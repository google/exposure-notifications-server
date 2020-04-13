package database

import (
	"cambio/pkg/logging"
	"cambio/pkg/model"
	"context"
	"fmt"

	"cloud.google.com/go/datastore"
)

func InsertInfections(ctx context.Context, infections []model.Infection) error {
	logger := logging.FromContext(ctx)

	client := Connection()
	if client == nil {
		return fmt.Errorf("unable to obtain database client")
	}

	// Using auto keys
	keys := make([]*datastore.Key, 0, len(infections))
	for range infections {
		keys = append(keys, datastore.IncompleteKey(model.InfectionTable, nil))
	}
	logger.Infof("Writing %v records", len(infections))

	_, err := client.PutMulti(ctx, keys, infections)

	return err
}
