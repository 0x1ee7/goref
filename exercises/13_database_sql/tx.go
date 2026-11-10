package orders

import (
	"context"
	"database/sql"
)

// WithTx runs fn inside a transaction on db.
//
// The transaction is committed when fn returns nil. If fn returns an error, or
// panics, the transaction is rolled back; a panic is re-raised after the
// rollback. A failing Commit is returned to the caller, not swallowed. If the
// rollback itself fails (other than because the transaction is already done),
// that error is joined with fn's error, so errors.Is still matches fn's error.
func WithTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	return errNotImplemented
}
