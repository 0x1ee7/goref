package orders_test

import (
	"database/sql"
	"errors"
	"testing"

	orders "goref/exercises/13_database_sql"
	"modernc.org/sqlite"
)

func TestPart2Migrate(t *testing.T) {
	t.Run("migrations are numbered 1..n", func(t *testing.T) {
		if len(orders.Migrations) == 0 {
			t.Fatal("Migrations is empty")
		}
		for i, m := range orders.Migrations {
			if m.Version != i+1 {
				t.Errorf("Migrations[%d].Version = %d, want %d", i, m.Version, i+1)
			}
		}
	})

	t.Run("fresh database reaches the latest version", func(t *testing.T) {
		db := openDB(t)
		got, err := orders.Migrate(t.Context(), db, orders.Migrations)
		if err != nil {
			t.Fatalf("Migrate: %v", err)
		}
		if want := len(orders.Migrations); got != want {
			t.Errorf("Migrate() = %d, want %d", got, want)
		}
		if v := userVersion(t, db); v != got {
			t.Errorf("user_version = %d, want %d", v, got)
		}
	})

	t.Run("creates the scenario tables", func(t *testing.T) {
		db := migratedDB(t)
		for _, table := range []string{"customers", "products", "orders", "order_items"} {
			var n int
			if err := db.QueryRow(
				"SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table,
			).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 1 {
				t.Errorf("table %q missing after migrations", table)
			}
		}
	})

	t.Run("creates explicit indexes on orders", func(t *testing.T) {
		db := migratedDB(t)
		var n int
		if err := db.QueryRow(
			"SELECT count(*) FROM sqlite_master WHERE type = 'index' AND tbl_name = 'orders' AND sql IS NOT NULL",
		).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n < 2 {
			t.Errorf("explicit indexes on orders = %d, want at least 2 (pagination and reporting)", n)
		}
	})

	t.Run("rerunning is a no-op", func(t *testing.T) {
		db := migratedDB(t)
		got, err := orders.Migrate(t.Context(), db, orders.Migrations)
		if err != nil {
			t.Fatalf("second Migrate: %v", err)
		}
		if want := len(orders.Migrations); got != want {
			t.Errorf("second Migrate() = %d, want %d", got, want)
		}
	})

	t.Run("applies only the missing versions", func(t *testing.T) {
		db := openDB(t)
		migs := testMigrations()
		if got, err := orders.Migrate(t.Context(), db, migs[:1]); err != nil || got != 1 {
			t.Fatalf("Migrate(v1) = %d, %v; want 1, nil", got, err)
		}
		if got, err := orders.Migrate(t.Context(), db, migs); err != nil || got != 2 {
			t.Fatalf("Migrate(v1,v2) = %d, %v; want 2, nil", got, err)
		}
		for _, table := range []string{"mig_a", "mig_b"} {
			if !tableExists(t, db, table) {
				t.Errorf("table %q missing", table)
			}
		}
	})

	t.Run("failed migration is atomic and chains the driver error", func(t *testing.T) {
		db := openDB(t)
		migs := testMigrations()
		broken := orders.Migration{
			Version: 2,
			Name:    "broken",
			Statements: []string{
				"CREATE TABLE mig_c (x INTEGER)",
				"CREATE TABLE broken ((",
			},
		}
		if _, err := orders.Migrate(t.Context(), db, []orders.Migration{migs[0], broken}); err == nil {
			t.Fatal("Migrate with broken v2 succeeded, want error")
		} else {
			if !errors.Is(err, orders.ErrMigration) {
				t.Errorf("error = %v, want wrapping ErrMigration", err)
			}
			var serr *sqlite.Error
			if !errors.As(err, &serr) {
				t.Errorf("error = %v, want driver *sqlite.Error in its chain", err)
			}
		}
		if v := userVersion(t, db); v != 1 {
			t.Errorf("user_version = %d, want 1 after failed v2", v)
		}
		if tableExists(t, db, "mig_c") {
			t.Error("mig_c exists: failed migration was not rolled back")
		}
	})

	t.Run("database newer than the migrations is rejected", func(t *testing.T) {
		db := openDB(t)
		migs := testMigrations()
		if _, err := orders.Migrate(t.Context(), db, migs); err != nil {
			t.Fatalf("Migrate: %v", err)
		}
		_, err := orders.Migrate(t.Context(), db, migs[:1])
		if !errors.Is(err, orders.ErrSchemaTooNew) {
			t.Errorf("Migrate(older set) error = %v, want ErrSchemaTooNew", err)
		}
		if v := userVersion(t, db); v != 2 {
			t.Errorf("user_version = %d, want 2 untouched", v)
		}
	})

	t.Run("non-contiguous versions are rejected before touching the database", func(t *testing.T) {
		migs := testMigrations()
		gap := []orders.Migration{migs[0], {Version: 3, Name: "gap", Statements: []string{"CREATE TABLE g (x INTEGER)"}}}
		shifted := []orders.Migration{{Version: 2, Name: "shifted", Statements: []string{"CREATE TABLE s (x INTEGER)"}}}
		for name, set := range map[string][]orders.Migration{"gap": gap, "shifted": shifted} {
			db := openDB(t)
			if _, err := orders.Migrate(t.Context(), db, set); !errors.Is(err, orders.ErrInvalidInput) {
				t.Errorf("%s: error = %v, want ErrInvalidInput", name, err)
			}
			if v := userVersion(t, db); v != 0 {
				t.Errorf("%s: user_version = %d, want 0", name, v)
			}
		}
	})

	t.Run("empty migration set on a fresh database", func(t *testing.T) {
		db := openDB(t)
		got, err := orders.Migrate(t.Context(), db, nil)
		if err != nil || got != 0 {
			t.Errorf("Migrate(nil) = %d, %v; want 0, nil", got, err)
		}
	})
}

func testMigrations() []orders.Migration {
	return []orders.Migration{
		{Version: 1, Name: "a", Statements: []string{"CREATE TABLE mig_a (x INTEGER)"}},
		{Version: 2, Name: "b", Statements: []string{"CREATE TABLE mig_b (x INTEGER)"}},
	}
}

func userVersion(t *testing.T, db *sql.DB) int {
	t.Helper()
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}
