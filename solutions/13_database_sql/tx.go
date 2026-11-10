package orders

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// WithTx runs fn inside a transaction on db.
//
// The transaction is committed when fn returns nil. If fn returns an error, or
// panics, the transaction is rolled back; a panic is re-raised after the
// rollback. A failing Commit is returned to the caller, not swallowed. If the
// rollback itself fails (other than because the transaction is already done),
// that error is joined with fn's error, so errors.Is still matches fn's error.
func WithTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("orders: begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
				err = errors.Join(err, fmt.Errorf("orders: rollback: %w", rbErr))
			}
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("orders: commit: %w", err)
	}
	return nil
}
