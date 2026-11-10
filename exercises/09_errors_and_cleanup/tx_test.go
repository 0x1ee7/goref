package support_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	support "goref/exercises/09_errors_and_cleanup"
)

func mustBegin(t *testing.T, s *support.Store) *support.Tx {
	t.Helper()
	tx, err := s.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if tx == nil {
		t.Fatal("Begin() returned nil transaction")
	}
	return tx
}

func mustGet(t *testing.T, s *support.Store, key string) string {
	t.Helper()
	v, ok := s.Get(key)
	if !ok {
		t.Fatalf("Store.Get(%q) not found, want a value", key)
	}
	return v
}

func TestPart4Store(t *testing.T) {
	t.Run("seed is copied", func(t *testing.T) {
		seed := map[string]string{"k": "orig"}
		s := support.NewStore(seed)
		seed["k"] = "changed"
		if v := mustGet(t, s, "k"); v != "orig" {
			t.Errorf("Get(k) = %q, want %q (store must copy its seed)", v, "orig")
		}
	})

	t.Run("nil seed is usable", func(t *testing.T) {
		s := support.NewStore(nil)
		tx := mustBegin(t, s)
		if err := tx.Set("a", "1"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit() error = %v", err)
		}
		if v := mustGet(t, s, "a"); v != "1" {
			t.Errorf("Get(a) = %q, want %q", v, "1")
		}
	})

	t.Run("snapshot is a copy", func(t *testing.T) {
		s := support.NewStore(map[string]string{"k": "v"})
		snap := s.Snapshot()
		if snap["k"] != "v" {
			t.Fatalf("Snapshot()[k] = %q, want %q", snap["k"], "v")
		}
		snap["k"] = "mutated"
		if v := mustGet(t, s, "k"); v != "v" {
			t.Errorf("Get(k) = %q after mutating snapshot, want %q", v, "v")
		}
	})

	t.Run("second Begin while active is ErrTxActive", func(t *testing.T) {
		s := support.NewStore(nil)
		tx1 := mustBegin(t, s)
		_, err := s.Begin()
		if !errors.Is(err, support.ErrTxActive) {
			t.Fatalf("second Begin() error = %v, want ErrTxActive", err)
		}
		if err := tx1.Rollback(); err != nil {
			t.Fatalf("Rollback() error = %v", err)
		}
		tx2, err := s.Begin()
		if err != nil || tx2 == nil {
			t.Errorf("Begin() after Rollback = (%v, %v), want a new transaction", tx2, err)
		}
	})
}

func TestPart4Tx(t *testing.T) {
	t.Run("sees own writes, commits them", func(t *testing.T) {
		s := support.NewStore(map[string]string{"a": "1"})
		tx := mustBegin(t, s)
		if err := tx.Set("a", "2"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if err := tx.Set("b", "3"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if v, err := tx.Get("a"); err != nil || v != "2" {
			t.Errorf("tx.Get(a) = (%q, %v), want (%q, nil)", v, err, "2")
		}
		if v := mustGet(t, s, "a"); v != "1" {
			t.Errorf("store Get(a) before commit = %q, want uncommitted isolation %q", v, "1")
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit() error = %v", err)
		}
		want := map[string]string{"a": "2", "b": "3"}
		if diff := cmp.Diff(want, s.Snapshot()); diff != "" {
			t.Errorf("Snapshot() after commit mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("delete hides key inside tx and commits removal", func(t *testing.T) {
		s := support.NewStore(map[string]string{"a": "1"})
		tx := mustBegin(t, s)
		if err := tx.Delete("a"); err != nil {
			t.Fatalf("Delete(a) error = %v", err)
		}
		if _, err := tx.Get("a"); !errors.Is(err, support.ErrKeyNotFound) {
			t.Errorf("tx.Get(a) after delete error = %v, want ErrKeyNotFound", err)
		}
		if err := tx.Delete("missing"); !errors.Is(err, support.ErrKeyNotFound) {
			t.Errorf("Delete(missing) error = %v, want ErrKeyNotFound", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit() error = %v", err)
		}
		if _, ok := s.Get("a"); ok {
			t.Errorf("Get(a) after committed delete found a value, want none")
		}
	})

	t.Run("rollback discards writes", func(t *testing.T) {
		s := support.NewStore(map[string]string{"a": "1"})
		tx := mustBegin(t, s)
		if err := tx.Set("a", "9"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if err := tx.Set("b", "2"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatalf("Rollback() error = %v", err)
		}
		want := map[string]string{"a": "1"}
		if diff := cmp.Diff(want, s.Snapshot()); diff != "" {
			t.Errorf("Snapshot() after rollback mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("finished tx rejects every operation with ErrTxDone", func(t *testing.T) {
		s := support.NewStore(nil)
		tx := mustBegin(t, s)
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit() error = %v", err)
		}
		ops := map[string]func() error{
			"Get":       func() error { _, err := tx.Get("a"); return err },
			"Set":       func() error { return tx.Set("a", "1") },
			"Delete":    func() error { return tx.Delete("a") },
			"Savepoint": func() error { return tx.Savepoint(func() error { return nil }) },
			"Commit":    tx.Commit,
			"Rollback":  tx.Rollback,
		}
		for name, op := range ops {
			t.Run(name, func(t *testing.T) {
				if err := op(); !errors.Is(err, support.ErrTxDone) {
					t.Errorf("%s() after commit error = %v, want ErrTxDone", name, err)
				}
			})
		}
	})

	t.Run("savepoint undoes failed work and keeps the tx open", func(t *testing.T) {
		boom := errors.New("boom")
		s := support.NewStore(map[string]string{"a": "x"})
		tx := mustBegin(t, s)
		err := tx.Savepoint(func() error {
			if err := tx.Set("a", "y"); err != nil {
				return err
			}
			if err := tx.Set("c", "z"); err != nil {
				return err
			}
			return boom
		})
		if !errors.Is(err, boom) {
			t.Fatalf("Savepoint() error = %v, want boom", err)
		}
		if v, err := tx.Get("a"); err != nil || v != "x" {
			t.Errorf("tx.Get(a) = (%q, %v), want (%q, nil)", v, err, "x")
		}
		if _, err := tx.Get("c"); !errors.Is(err, support.ErrKeyNotFound) {
			t.Errorf("tx.Get(c) error = %v, want ErrKeyNotFound", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit() after savepoint error = %v, want nil (tx stays open)", err)
		}
	})

	t.Run("ErrRollbackSavepoint is swallowed after undo", func(t *testing.T) {
		s := support.NewStore(map[string]string{"a": "x"})
		tx := mustBegin(t, s)
		err := tx.Savepoint(func() error {
			if err := tx.Set("a", "y"); err != nil {
				return err
			}
			return support.ErrRollbackSavepoint
		})
		if err != nil {
			t.Fatalf("Savepoint() error = %v, want nil", err)
		}
		if v, err := tx.Get("a"); err != nil || v != "x" {
			t.Errorf("tx.Get(a) = (%q, %v), want (%q, nil)", v, err, "x")
		}
	})

	t.Run("savepoint panic undoes writes and propagates", func(t *testing.T) {
		s := support.NewStore(map[string]string{"a": "x"})
		tx := mustBegin(t, s)
		var recovered any
		func() {
			defer func() { recovered = recover() }()
			_ = tx.Savepoint(func() error {
				if err := tx.Set("a", "y"); err != nil {
					return err
				}
				panic("bang")
			})
		}()
		if recovered != "bang" {
			t.Fatalf("recovered = %v, want the panic %q to propagate", recovered, "bang")
		}
		if v, err := tx.Get("a"); err != nil || v != "x" {
			t.Errorf("tx.Get(a) after panic = (%q, %v), want (%q, nil)", v, err, "x")
		}
		if err := tx.Commit(); err != nil {
			t.Errorf("Commit() after panic = %v, want nil (tx stays open)", err)
		}
	})
}

func TestPart4Update(t *testing.T) {
	t.Run("nil commits", func(t *testing.T) {
		s := support.NewStore(map[string]string{"a": "1"})
		err := support.Update(s, func(tx *support.Tx) error {
			return tx.Set("a", "new")
		})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if v := mustGet(t, s, "a"); v != "new" {
			t.Errorf("Get(a) = %q, want %q", v, "new")
		}
	})

	t.Run("error rolls back and is returned unchanged", func(t *testing.T) {
		boom := errors.New("boom")
		s := support.NewStore(map[string]string{"a": "1"})
		err := support.Update(s, func(tx *support.Tx) error {
			if err := tx.Set("a", "new"); err != nil {
				return err
			}
			return boom
		})
		if err != boom {
			t.Fatalf("Update() error = %v, want the fn error unchanged", err)
		}
		if v := mustGet(t, s, "a"); v != "1" {
			t.Errorf("Get(a) = %q after rollback, want %q", v, "1")
		}
	})

	t.Run("panic rolls back, releases the store, and propagates", func(t *testing.T) {
		s := support.NewStore(map[string]string{"a": "1"})
		var recovered any
		func() {
			defer func() { recovered = recover() }()
			_ = support.Update(s, func(tx *support.Tx) error {
				if err := tx.Set("a", "new"); err != nil {
					return err
				}
				panic("bang")
			})
		}()
		if recovered != "bang" {
			t.Fatalf("recovered = %v, want the panic %q to propagate", recovered, "bang")
		}
		if v := mustGet(t, s, "a"); v != "1" {
			t.Errorf("Get(a) = %q after panic, want %q", v, "1")
		}
		tx, err := s.Begin()
		if err != nil || tx == nil {
			t.Errorf("Begin() after panic = (%v, %v), want a new transaction", tx, err)
		}
	})
}
