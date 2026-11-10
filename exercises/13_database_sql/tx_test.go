package orders_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	orders "goref/exercises/13_database_sql"
)

func TestPart3WithTx(t *testing.T) {
	sentinel := errors.New("sentinel")

	t.Run("commits when fn returns nil", func(t *testing.T) {
		db := kvDB(t)
		err := orders.WithTx(t.Context(), db, func(tx *sql.Tx) error {
			return insertKV(tx, "a")
		})
		if err != nil {
			t.Fatalf("WithTx: %v", err)
		}
		if n := countKV(t, db); n != 1 {
			t.Errorf("rows = %d, want 1", n)
		}
	})

	t.Run("rolls back and returns fn's error unchanged", func(t *testing.T) {
		db := kvDB(t)
		err := orders.WithTx(t.Context(), db, func(tx *sql.Tx) error {
			if err := insertKV(tx, "a"); err != nil {
				return err
			}
			return sentinel
		})
		if err != sentinel {
			t.Errorf("WithTx error = %v, want sentinel itself", err)
		}
		if n := countKV(t, db); n != 0 {
			t.Errorf("rows = %d, want 0 after rollback", n)
		}
	})

	t.Run("rolls back on panic and re-raises", func(t *testing.T) {
		db := kvDB(t)
		func() {
			defer func() {
				if r := recover(); r != "boom" {
					t.Errorf("recovered %v, want the original panic value \"boom\"", r)
				}
			}()
			_ = orders.WithTx(t.Context(), db, func(tx *sql.Tx) error {
				if err := insertKV(tx, "a"); err != nil {
					t.Errorf("insert: %v", err)
				}
				panic("boom")
			})
			t.Error("WithTx returned normally after a panic in fn")
		}()
		if n := countKV(t, db); n != 0 {
			t.Errorf("rows = %d, want 0 after panic rollback", n)
		}
	})

	t.Run("commit failure is surfaced", func(t *testing.T) {
		db := kvDB(t)
		err := orders.WithTx(t.Context(), db, func(tx *sql.Tx) error {
			if err := tx.Commit(); err != nil {
				t.Errorf("early commit: %v", err)
			}
			return nil
		})
		if !errors.Is(err, sql.ErrTxDone) {
			t.Errorf("WithTx error = %v, want the commit failure (sql.ErrTxDone)", err)
		}
	})

	t.Run("rollback failure is joined with fn's error", func(t *testing.T) {
		db := kvDB(t)
		err := orders.WithTx(t.Context(), db, func(tx *sql.Tx) error {
			if err := tx.Rollback(); err != nil {
				t.Errorf("early rollback: %v", err)
			}
			return sentinel
		})
		if !errors.Is(err, sentinel) {
			t.Errorf("WithTx error = %v, want it to match fn's error", err)
		}
	})

	t.Run("cancelled context prevents fn from running", func(t *testing.T) {
		db := kvDB(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		called := false
		err := orders.WithTx(ctx, db, func(tx *sql.Tx) error {
			called = true
			return nil
		})
		if err == nil {
			t.Error("WithTx with cancelled context succeeded, want error")
		}
		if called {
			t.Error("fn ran with a cancelled context")
		}
	})
}

func kvDB(t *testing.T) *sql.DB {
	t.Helper()
	db := openDB(t)
	mustExec(t, db, "CREATE TABLE kv (k TEXT PRIMARY KEY)")
	return db
}

func insertKV(tx *sql.Tx, k string) error {
	_, err := tx.Exec("INSERT INTO kv (k) VALUES (?)", k)
	return err
}

func countKV(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT count(*) FROM kv").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
