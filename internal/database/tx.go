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
	"context"
	"fmt"

	"github.com/googlepartners/exposure-notifications/internal/logging"

	pgx "github.com/jackc/pgx/v4"
)

// finish is a convenience function that can be deferred to commit or rollback a transaction according to boolean commit flag. err will be populated if there is an error.
func finishTx(ctx context.Context, tx pgx.Tx, commit *bool, err *error) {
	if *commit {
		if err1 := tx.Commit(ctx); err1 != nil {
			*err = fmt.Errorf("failed to commit: %v", err1)
		}
	} else {
		if err1 := tx.Rollback(ctx); err1 != nil {
			*err = fmt.Errorf("failed to rollback: %v", err1)
		} else {
			logger := logging.FromContext(ctx)
			logger.Infof("Rolling back.")
		}
	}
}
