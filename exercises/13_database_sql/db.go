// Package orders is the persistence layer of a small order-management backend
// on SQLite (modernc.org/sqlite, pure Go, driver name "sqlite"). The spec is in
// README.md; the tests in *_test.go are the executable contract.
package orders

import (
	"context"
	"database/sql"
	"errors"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

var errNotImplemented = errors.New("not implemented")

// Sentinel errors. Wrap them with %w; callers match them with errors.Is.
var (
	// ErrNotFound means the requested row does not exist, or that an order
	// references a customer that does not exist.
	ErrNotFound = errors.New("orders: not found")

	// ErrInvalidInput means an argument failed validation: empty search term,
	// quantity below 1, negative price, unknown status, malformed cursor,
	// non-positive batch size, negative offset or limit, and so on.
	ErrInvalidInput = errors.New("orders: invalid input")

	// ErrDuplicateEmail means another customer already uses the email,
	// compared case-insensitively.
	ErrDuplicateEmail = errors.New("orders: duplicate email")
)

// DSN returns the modernc.org/sqlite connection string for the database file
// at path. The string enables, for every connection the pool opens:
//
//   - foreign key enforcement (off by default in SQLite);
//   - a busy timeout of 5000 ms, so a locked database is waited for instead of
//     failing immediately with SQLITE_BUSY;
//   - WAL journal mode (persistent in the file);
//   - BEGIN IMMEDIATE for every transaction started by database/sql, so a
//     transaction takes the write lock up front.
func DSN(path string) string {
	return ""
}

// Open opens (creating if needed) the database at path and returns a pool
// configured for SQLite:
//
//   - MaxOpenConns is 1: SQLite allows one writer at a time and every pooled
//     connection competes for the same file lock. One connection serializes
//     writers inside the process instead of letting them collide with
//     SQLITE_BUSY.
//   - The pool is verified with a ping bounded by ctx. On failure the pool is
//     closed and the error is returned wrapped.
//
// The caller owns the returned *sql.DB and must Close it.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	return nil, errNotImplemented
}

// Money is an amount in minor units (cents). Never use float64 for money.
type Money int64

// String formats m as a decimal amount with exactly two fraction digits:
// 1234 becomes "12.34", 5 becomes "0.05", -5 becomes "-0.05". No thousands
// separators and no currency symbol.
func (m Money) String() string {
	return ""
}
