package orders

import (
	"context"
	"database/sql"
	"errors"
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
var Migrations []Migration

// Migrate brings db up to the newest version in migrations and returns the
// resulting schema version (PRAGMA user_version).
//
// It returns an error wrapping ErrInvalidInput when migrations are not numbered
// 1..n in order, and ErrSchemaTooNew when the database is already past the last
// migration. Each migration whose version is above the current one runs in its
// own transaction together with the user_version bump, so a failure leaves the
// schema and the version consistent. Re-running is a no-op.
func Migrate(ctx context.Context, db *sql.DB, migrations []Migration) (int, error) {
	return 0, errNotImplemented
}
