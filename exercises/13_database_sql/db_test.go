package orders_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	orders "goref/exercises/13_database_sql"
)

func TestPart1DSN(t *testing.T) {
	dsn := orders.DSN(filepath.Join(t.TempDir(), "shop.db"))
	for _, want := range []string{
		"_pragma=foreign_keys(1)",
		"_pragma=busy_timeout(5000)",
		"_pragma=journal_mode(WAL)",
		"_txlock=immediate",
	} {
		if !strings.Contains(dsn, want) {
			t.Errorf("DSN() = %q, want it to contain %q", dsn, want)
		}
	}
}

func TestPart1Open(t *testing.T) {
	t.Run("pool is capped at one connection", func(t *testing.T) {
		db := openDB(t)
		if got := db.Stats().MaxOpenConnections; got != 1 {
			t.Errorf("MaxOpenConnections = %d, want 1", got)
		}
	})

	t.Run("pragmas are applied", func(t *testing.T) {
		db := openDB(t)
		var fk, busy int
		var journal string
		if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow("PRAGMA busy_timeout").Scan(&busy); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil {
			t.Fatal(err)
		}
		if fk != 1 {
			t.Errorf("foreign_keys = %d, want 1", fk)
		}
		if busy != 5000 {
			t.Errorf("busy_timeout = %d, want 5000", busy)
		}
		if strings.ToLower(journal) != "wal" {
			t.Errorf("journal_mode = %q, want wal", journal)
		}
	})

	t.Run("foreign keys are enforced", func(t *testing.T) {
		db := openDB(t)
		mustExec(t, db, "CREATE TABLE parent (id INTEGER PRIMARY KEY)")
		mustExec(t, db, "CREATE TABLE child (parent_id INTEGER REFERENCES parent(id))")
		if _, err := db.Exec("INSERT INTO child (parent_id) VALUES (99)"); err == nil {
			t.Error("insert with dangling foreign key succeeded, want error")
		}
	})

	t.Run("data survives reopening the file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "reopen.db")
		db1, err := orders.Open(t.Context(), path)
		if err != nil {
			t.Fatalf("first Open: %v", err)
		}
		mustExec(t, db1, "CREATE TABLE kv (v INTEGER)")
		mustExec(t, db1, "INSERT INTO kv (v) VALUES (42)")
		if err := db1.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		db2, err := orders.Open(t.Context(), path)
		if err != nil {
			t.Fatalf("second Open: %v", err)
		}
		t.Cleanup(func() { _ = db2.Close() })
		var v int
		if err := db2.QueryRow("SELECT v FROM kv").Scan(&v); err != nil {
			t.Fatalf("read after reopen: %v", err)
		}
		if v != 42 {
			t.Errorf("v = %d, want 42", v)
		}
	})

	t.Run("path with spaces works", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "my shop")
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		db, err := orders.Open(t.Context(), filepath.Join(dir, "shop.db"))
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		t.Cleanup(func() { _ = db.Close() })
		if err := db.PingContext(t.Context()); err != nil {
			t.Fatalf("Ping: %v", err)
		}
	})

	t.Run("unopenable path fails", func(t *testing.T) {
		ok, err := orders.Open(t.Context(), filepath.Join(t.TempDir(), "shop.db"))
		if err != nil {
			t.Fatalf("Open of a valid path: %v", err)
		}
		_ = ok.Close()
		path := filepath.Join(t.TempDir(), "missing", "shop.db")
		db, err := orders.Open(t.Context(), path)
		if err == nil {
			_ = db.Close()
			t.Fatal("Open in a missing directory succeeded, want error")
		}
	})

	t.Run("cancelled context fails", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		db, err := orders.Open(ctx, filepath.Join(t.TempDir(), "shop.db"))
		if err == nil {
			_ = db.Close()
			t.Fatal("Open with cancelled context succeeded, want error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Open error = %v, want wrapping context.Canceled", err)
		}
	})
}

func TestPart1Money(t *testing.T) {
	for _, tc := range []struct {
		m    orders.Money
		want string
	}{
		{0, "0.00"},
		{5, "0.05"},
		{1234, "12.34"},
		{-5, "-0.05"},
		{-1250, "-12.50"},
		{100000000, "1000000.00"},
	} {
		if got := tc.m.String(); got != tc.want {
			t.Errorf("Money(%d).String() = %q, want %q", int64(tc.m), got, tc.want)
		}
	}
}

func mustExec(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	if _, err := db.Exec(query); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}
