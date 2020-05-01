package database

import (
	"context"
	"fmt"

	pgx "github.com/jackc/pgx/v4"
)

// inTx runs the given function f within a transaction with isolation level isoLevel.
func (db *DB) inTx(ctx context.Context, isoLevel pgx.TxIsoLevel, f func(tx pgx.Tx) error) error {
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquiring connection: %v", err)
	}
	defer conn.Release()

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: isoLevel})
	if err != nil {
		return fmt.Errorf("starting transaction: %v", err)
	}

	if err := f(tx); err != nil {
		if err1 := tx.Rollback(ctx); err1 != nil {
			return fmt.Errorf("rolling back transaction: %v (original error: %v)", err1, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %v", err)
	}
	return nil
}
