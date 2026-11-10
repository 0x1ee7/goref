package orders

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrMigration wraps every failure to apply a migration. The underlying driver
// error is also wrapped, so both errors.Is(err, ErrMigration) and the driver
// error are visible.
var ErrMigration = errors.New("orders: migration failed")

// ErrSchemaTooNew means the database was migrated by a newer binary than this
// one knows about.
var ErrSchemaTooNew = errors.New("orders: database schema is newer than the migrations")

// Migration is one versioned, atomically applied schema change.
type Migration struct {
	Version    int      // 1-based, contiguous
	Name       string   // human-readable, for error messages
	Statements []string // executed in order inside one transaction
}

// Migrations is the schema history of the order backend: v1 creates the
// tables, v2 adds the indexes used by pagination and reports.
var Migrations = []Migration{
	{
		Version: 1,
		Name:    "tables",
		Statements: []string{
			`CREATE TABLE customers (
				id         INTEGER PRIMARY KEY,
				email      TEXT NOT NULL UNIQUE COLLATE NOCASE,
				name       TEXT NOT NULL,
				phone      TEXT,
				created_at INTEGER NOT NULL
			) STRICT`,
			`CREATE TABLE products (
				sku         TEXT PRIMARY KEY,
				name        TEXT NOT NULL,
				price_cents INTEGER NOT NULL CHECK (price_cents >= 0)
			) STRICT`,
			`CREATE TABLE orders (
				id          INTEGER PRIMARY KEY,
				customer_id INTEGER NOT NULL REFERENCES customers(id),
				placed_at   INTEGER NOT NULL,
				status      TEXT NOT NULL CHECK (status IN ('pending', 'paid', 'shipped', 'cancelled'))
			) STRICT`,
			`CREATE TABLE order_items (
				order_id         INTEGER NOT NULL REFERENCES orders(id),
				line_no          INTEGER NOT NULL,
				sku              TEXT NOT NULL REFERENCES products(sku),
				quantity         INTEGER NOT NULL CHECK (quantity >= 1),
				unit_price_cents INTEGER NOT NULL CHECK (unit_price_cents >= 0),
				PRIMARY KEY (order_id, line_no)
			) STRICT`,
		},
	},
	{
		Version: 2,
		Name:    "indexes",
		Statements: []string{
			`CREATE INDEX orders_placed_idx ON orders (placed_at, id)`,
			`CREATE INDEX orders_customer_placed_idx ON orders (customer_id, placed_at, id)`,
		},
	},
}

// Migrate brings db up to the newest version in migrations and returns the
// resulting schema version (PRAGMA user_version).
//
// It returns an error wrapping ErrInvalidInput when migrations are not numbered
// 1..n in order, and ErrSchemaTooNew when the database is already past the last
// migration. Each migration whose version is above the current one runs in its
// own transaction together with the user_version bump, so a failure leaves the
// schema and the version consistent. Re-running is a no-op.
func Migrate(ctx context.Context, db *sql.DB, migrations []Migration) (int, error) {
	for i, m := range migrations {
		if m.Version != i+1 {
			return 0, fmt.Errorf("%w: migration %q is version %d at position %d, want %d",
				ErrInvalidInput, m.Name, m.Version, i+1, i+1)
		}
	}
	current, err := userVersion(ctx, db)
	if err != nil {
		return 0, err
	}
	if current > len(migrations) {
		return current, fmt.Errorf("%w: database is at version %d, newest known is %d",
			ErrSchemaTooNew, current, len(migrations))
	}
	for _, m := range migrations[current:] {
		if err := applyMigration(ctx, db, m); err != nil {
			return current, err
		}
		current = m.Version
	}
	return current, nil
}

func userVersion(ctx context.Context, db *sql.DB) (int, error) {
	var v int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		return 0, fmt.Errorf("%w: read user_version: %w", ErrMigration, err)
	}
	return v, nil
}

func applyMigration(ctx context.Context, db *sql.DB, m Migration) error {
	return WithTx(ctx, db, func(tx *sql.Tx) error {
		for _, stmt := range m.Statements {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("%w: v%d %s: %w", ErrMigration, m.Version, m.Name, err)
			}
		}
		// PRAGMA arguments cannot be bound as parameters; m.Version is an int,
		// so formatting it into the statement is safe.
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", m.Version)); err != nil {
			return fmt.Errorf("%w: v%d %s: set user_version: %w", ErrMigration, m.Version, m.Name, err)
		}
		return nil
	})
}
