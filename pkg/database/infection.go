package database

import (
	"cambio/pkg/logging"
	"cambio/pkg/model"
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/iterator"
)

const (
	defaultFetchInfectionsPageSize = 1000
)

// InsertInfections adds a set of infections to the database.
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

func GetInfections(ctx context.Context) ([]model.Infection, error) {
	client := Connection()
	if client == nil {
		return nil, fmt.Errorf("unable to obtain database client")
	}

	var infections []model.Infection
	q := datastore.NewQuery("infection").Limit(10) // TODO(guray): add filter by time, plumbed through from request (use filter.go)
	if _, err := client.GetAll(ctx, q, &infections); err != nil {
		return nil, fmt.Errorf("unable to fetch infections")
	}
	return infections, nil
}

// FetchInfectionsCriteria is criteria to query infections.
type FetchInfectionsCriteria struct {
	IncludeRegions []string
	SinceTimestamp time.Time
	UntilTimestamp time.Time
	LastCursor     string
}

// InfectionIterator iterates over a set of infections.
type InfectionIterator interface {
	// Next returns an infection and a flag indicating if the iterator is done (the infection will be nil when done==true).
	Next() (infection *model.Infection, done bool, err error)
	// Cursor returns a string that can be passed as LastCursor in FetchInfectionsCriteria when generating another iterator.
	Cursor() (string, error)
}

type datastoreInfectionIterator struct {
	it *datastore.Iterator
}

func (i *datastoreInfectionIterator) Next() (*model.Infection, bool, error) {
	var inf model.Infection
	if _, err := i.it.Next(&inf); err != nil {
		if err == iterator.Done {
			return nil, true, nil
		}
		return nil, false, err
	}
	return &inf, false, nil
}

func (i *datastoreInfectionIterator) Cursor() (string, error) {
	c, err := i.it.Cursor()
	if err != nil {
		return "", err
	}
	return c.String(), nil
}

// IterateInfections returns an iterator for infections meeting the criteria.
func IterateInfections(ctx context.Context, criteria FetchInfectionsCriteria) (InfectionIterator, error) {
	query, err := fetchQuery(criteria)
	if err != nil {
		return nil, fmt.Errorf("generating query: %v", err)
	}

	client := Connection()
	if client == nil {
		return nil, fmt.Errorf("unable to obtain database client")
	}

	return &datastoreInfectionIterator{it: client.Run(ctx, query)}, nil
}

func fetchQuery(criteria FetchInfectionsCriteria) (*datastore.Query, error) {
	q := datastore.NewQuery(model.InfectionTable)

	if len(criteria.IncludeRegions) > 0 {
		q = q.Filter("Country IN", criteria.IncludeRegions)
	}

	if !criteria.SinceTimestamp.IsZero() {
		q = q.Filter("BatchTimestamp >", criteria.SinceTimestamp)
	}

	if !criteria.UntilTimestamp.IsZero() {
		q = q.Filter("BatchTimestamp <=", criteria.UntilTimestamp)
	}

	q = q.Order("BatchTimestamp")

	if criteria.LastCursor != "" {
		c, err := datastore.DecodeCursor(criteria.LastCursor)
		if err != nil {
			return nil, fmt.Errorf("decoding cursor: %v", err)
		}
		q = q.Start(c)
	}

	return q, nil
}
