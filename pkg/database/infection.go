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

package database

import (
	"cambio/pkg/logging"
	"cambio/pkg/model"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// InfectionTable holds uploaded infected keys.
	InfectionTable = "infection"

	defaultFetchInfectionsPageSize = 1000

	// InsertInfectionsBatchSize is the maximum number of infections that can be inserted at once.
	InsertInfectionsBatchSize = 500
)

// makeInfectionDatastoreKey turns a ExposureKey (16 bytes) into a datastore key,
// which is just the standard base64 encoding for those bytes.
func makeInfectionDatastoreKey(ExposureKey []byte) *datastore.Key {
	return datastore.NameKey(InfectionTable, base64.StdEncoding.EncodeToString(ExposureKey), nil)
}

// InsertInfections adds a set of infections to the database.
func InsertInfections(ctx context.Context, infections []model.Infection) error {
	logger := logging.FromContext(ctx)

	client := Connection()
	if client == nil {
		return fmt.Errorf("unable to obtain database client")
	}

	if len(infections) > InsertInfectionsBatchSize {
		return fmt.Errorf("batch size %d too large (maximum %d)", len(infections), InsertInfectionsBatchSize)
	}

	// Use datastore's batch loading functionality. By bundling inserts in
	// a non-transactional batch - each individual update will be transactional,
	// but we don't need to take locks over the entier enntity group.
	mutations := make([]*datastore.Mutation, 0, len(infections))
	for _, inf := range infections {
		inf.K = makeInfectionDatastoreKey(inf.ExposureKey)
		mutations = append(mutations, datastore.NewInsert(inf.K, &inf))
	}

	logger.Infof("Writing %v records", len(mutations))
	_, err := client.Mutate(ctx, mutations...)
	if err != nil {
		logger.Errorf("client.Mutate: %v", err)
		var mError datastore.MultiError
		if status.Code(err) == codes.AlreadyExists {
			logger.Infof("datastore library bug - didn't get multi-error, but did get codes.AlreadyExists.")
		} else if errors.As(err, &mError) {
			duplicateKeys := 0
			for _, e := range mError {
				if status.Code(e) != codes.AlreadyExists {
					return err
				} else {
					duplicateKeys++
				}
			}
			logger.Infof("Dropped %v duplicate keys in the batch of %v keys", duplicateKeys, len(mutations))
		}
	}

	return nil
}

// FetchInfectionsCriteria is criteria to query infections.
type FetchInfectionsCriteria struct {
	IncludeRegions []string
	SinceTimestamp time.Time
	UntilTimestamp time.Time
	LastCursor     string

	// OnlyLocalProvenance indicates that only infections with LocalProvenance=true will be returned.
	OnlyLocalProvenance bool
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
	logger := logging.FromContext(ctx)
	query, err := fetchQuery(criteria)
	if err != nil {
		return nil, fmt.Errorf("generating query: %v", err)
	}
	logger.Debugf("Querying with %#v", query)

	client := Connection()
	if client == nil {
		return nil, fmt.Errorf("unable to obtain database client")
	}

	return &datastoreInfectionIterator{it: client.Run(ctx, query)}, nil
}

func fetchQuery(criteria FetchInfectionsCriteria) (*datastore.Query, error) {
	q := datastore.NewQuery(InfectionTable)

	if len(criteria.IncludeRegions) > 1 {
		return nil, errors.New("datastore cannot filter on multiple regions")
	}
	if len(criteria.IncludeRegions) == 1 {
		q = q.Filter("region =", criteria.IncludeRegions[0])
	}

	if !criteria.SinceTimestamp.IsZero() {
		q = q.Filter("createdAt >", criteria.SinceTimestamp)
	}

	if !criteria.UntilTimestamp.IsZero() {
		q = q.Filter("createdAt <=", criteria.UntilTimestamp)
	}

	if criteria.OnlyLocalProvenance {
		q = q.Filter("localProvenance =", true)
	}

	q = q.Order("createdAt")

	if criteria.LastCursor != "" {
		c, err := datastore.DecodeCursor(criteria.LastCursor)
		if err != nil {
			return nil, fmt.Errorf("decoding cursor: %v", err)
		}
		q = q.Start(c)
	}

	return q, nil
}
