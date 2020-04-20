package database

import (
	"cambio/pkg/model"
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/datastore"
)

var (
	// ErrQueryNotFound indicates that the requested federation query was not found in the database.
	ErrQueryNotFound = errors.New("query not found")
)

// FinalizeSyncFn is used to finalize a historical sync record.
type FinalizeSyncFn func(completed, maxTimestamp time.Time, totalInserted int) error

// GetFederationQuery returns a query for given queryID. If not found, ErrQueryNotFound will be returned.
func GetFederationQuery(ctx context.Context, queryID string) (*model.FederationQuery, error) {
	client := Connection()
	if client == nil {
		return nil, fmt.Errorf("unable to obtain database client")
	}

	key := federationQueryKey(queryID)

	var query model.FederationQuery
	if err := client.Get(ctx, key, &query); err != nil {
		if err == datastore.ErrNoSuchEntity {
			return nil, ErrQueryNotFound
		}
		return nil, fmt.Errorf("getting query %q: %v", queryID, err)
	}
	return &query, nil
}

// AddFederationQuery adds a FederationQuery entity. It will overwrite a query with matching queryID if it exists.
func AddFederationQuery(ctx context.Context, queryID string, query *model.FederationQuery) error {
	client := Connection()
	if client == nil {
		return fmt.Errorf("unable to obtain database client")
	}

	key := federationQueryKey(queryID)

	if _, err := client.Put(ctx, key, query); err != nil {
		return fmt.Errorf("putting federation query: %v", err)
	}
	return nil
}

// StartFederationSync stores a historical record of a query sync starting. It returns a FederationSync key, and a FinalizeSyncFn that must be invoked to finalize the historical record.
func StartFederationSync(ctx context.Context, query *model.FederationQuery) (*datastore.Key, FinalizeSyncFn, error) {
	client := Connection()
	if client == nil {
		return nil, nil, fmt.Errorf("unable to obtain database client")
	}

	key := federationSyncKey(query)
	if _, err := client.Put(ctx, key, &model.FederationSync{Started: time.Now()}); err != nil {
		return nil, nil, fmt.Errorf("putting initial sync entity for %s: %v", key, err)
	}

	finalize := func(completed, maxTimestamp time.Time, totalInserted int) error {
		_, err := client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {

			// Fetch the query record, update the max timestamp for the next batch.
			queryKey := key.Parent
			var query model.FederationQuery
			if errg := tx.Get(queryKey, &query); errg != nil {
				return fmt.Errorf("getting query %s: %v", queryKey, errg)
			}
			query.LastTimestamp = maxTimestamp
			if _, errp := tx.Put(queryKey, query); errp != nil {
				return fmt.Errorf("putting updated query %s: %v", queryKey, errp)
			}

			// Fetch the sync record, update the statistics.
			var sync model.FederationSync
			if errg := tx.Get(key, &sync); errg != nil {
				return fmt.Errorf("getting sync entity %s: %v", key, errg)
			}
			sync.Completed = completed
			sync.Insertions = totalInserted
			sync.MaxTimestamp = maxTimestamp
			if _, errp := tx.Put(key, &sync); errp != nil {
				return fmt.Errorf("putting updated sync entity %s: %v", key, errp)
			}

			return nil
		})
		if err != nil {
			return err
		}
		return nil
	}
	return key, finalize, nil
}

func federationQueryKey(queryID string) *datastore.Key {
	return datastore.NameKey(model.FederationQueryTable, queryID, nil)
}

func federationSyncKey(query *model.FederationQuery) *datastore.Key {
	// FederationSync keys have ancestor of FederationQuery so they can be updated together in transaction.
	return datastore.IncompleteKey(model.FederationSyncTable, query.K)
}
