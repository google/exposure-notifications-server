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
