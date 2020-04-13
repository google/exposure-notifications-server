// Package
package database

import (
	"context"
	"fmt"
	"log"

	"cloud.google.com/go/datastore"
)

var (
	client *datastore.Client = nil
)

func Initialize() error {
	if client != nil {
		return fmt.Errorf("database connection already initialized")
	}

	ctx := context.Background()
	var err error
	client, err = datastore.NewClient(ctx, "" /* projectID */)
	if err != nil {
		return err
	}

	log.Printf("established connection to cloud datastore")
	return nil
}

func Connection() *datastore.Client {
	return client
}
