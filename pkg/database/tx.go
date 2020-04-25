package database

import (
	"cambio/pkg/logging"
	"context"
	"fmt"

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
