package orders_test

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	orders "goref/exercises/13_database_sql"
)

// openDB opens a fresh database under t.TempDir and closes it at cleanup.
func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := orders.Open(t.Context(), filepath.Join(t.TempDir(), "shop.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close: %v", err)
		}
	})
	return db
}

// migratedDB is openDB plus the production migrations applied.
func migratedDB(t *testing.T) *sql.DB {
	t.Helper()
	db := openDB(t)
	if _, err := orders.Migrate(t.Context(), db, orders.Migrations); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

func mustTime(t testing.TB, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return v
}

// seedCatalogue inserts BOOK-1 (4999), MUG-7 (1250) and CABLE-2 (899).
func seedCatalogue(t *testing.T, db *sql.DB) {
	t.Helper()
	repo := orders.NewProductRepo(db)
	for _, p := range []orders.Product{
		{SKU: "BOOK-1", Name: "Go in Practice", Price: 4999},
		{SKU: "MUG-7", Name: "Coffee Mug", Price: 1250},
		{SKU: "CABLE-2", Name: "USB-C Cable", Price: 899},
	} {
		if err := repo.Upsert(t.Context(), p); err != nil {
			t.Fatalf("seed product %s: %v", p.SKU, err)
		}
	}
}

func addCustomer(t *testing.T, db *sql.DB, email, name string) orders.Customer {
	t.Helper()
	c, err := orders.NewCustomerRepo(db).Add(t.Context(), orders.NewCustomer{
		Email:     email,
		Name:      name,
		CreatedAt: mustTime(t, "2026-01-01T00:00:00Z"),
	})
	if err != nil {
		t.Fatalf("seed customer %s: %v", email, err)
	}
	return c
}

func placeAt(t *testing.T, db *sql.DB, customerID int64, at string, lines ...orders.Line) orders.Order {
	t.Helper()
	o, err := orders.NewOrderRepo(db).Place(t.Context(), customerID, mustTime(t, at), lines)
	if err != nil {
		t.Fatalf("seed order at %s: %v", at, err)
	}
	return o
}
