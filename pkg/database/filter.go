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
