package orders_test

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"
	"testing"

	orders "goref/exercises/13_database_sql"

	"github.com/google/go-cmp/cmp"
)

func productRows(n int, price func(i int) orders.Money) []orders.Product {
	rows := make([]orders.Product, n)
	for i := range rows {
		rows[i] = orders.Product{
			SKU:   fmt.Sprintf("SKU-%05d", i),
			Name:  fmt.Sprintf("Item %d", i),
			Price: price(i),
		}
	}
	return rows
}

func TestPart5ImportProducts(t *testing.T) {
	t.Run("imports every row", func(t *testing.T) {
		db := migratedDB(t)
		rows := productRows(7, func(i int) orders.Money { return orders.Money(100 + i) })
		n, err := orders.ImportProducts(t.Context(), db, slices.Values(rows), 3)
		if err != nil {
			t.Fatalf("ImportProducts: %v", err)
		}
		if n != len(rows) {
			t.Errorf("processed = %d, want %d", n, len(rows))
		}
		repo := orders.NewProductRepo(db)
		for _, want := range rows {
			got, err := repo.Get(t.Context(), want.SKU)
			if err != nil {
				t.Errorf("Get(%s): %v", want.SKU, err)
				continue
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("row %s mismatch (-want +got):\n%s", want.SKU, diff)
			}
		}
	})

	t.Run("re-import updates existing SKUs without duplicating", func(t *testing.T) {
		db := migratedDB(t)
		first := productRows(4, func(int) orders.Money { return 100 })
		if _, err := orders.ImportProducts(t.Context(), db, slices.Values(first), 2); err != nil {
			t.Fatalf("first import: %v", err)
		}
		second := productRows(4, func(int) orders.Money { return 250 })
		n, err := orders.ImportProducts(t.Context(), db, slices.Values(second), 2)
		if err != nil || n != 4 {
			t.Fatalf("second import = %d, %v; want 4, nil", n, err)
		}
		got, err := orders.NewProductRepo(db).Get(t.Context(), second[0].SKU)
		if err != nil || got.Price != 250 {
			t.Errorf("Get after re-import = %+v, %v; want price 250", got, err)
		}
		var count int
		if err := db.QueryRow("SELECT count(*) FROM products").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 4 {
			t.Errorf("products = %d, want 4 (no duplicates)", count)
		}
	})

	t.Run("empty input imports nothing", func(t *testing.T) {
		db := migratedDB(t)
		n, err := orders.ImportProducts(t.Context(), db, slices.Values([]orders.Product(nil)), 5)
		if err != nil || n != 0 {
			t.Errorf("ImportProducts(empty) = %d, %v; want 0, nil", n, err)
		}
	})

	t.Run("batch size below one is invalid", func(t *testing.T) {
		db := migratedDB(t)
		rows := productRows(2, func(int) orders.Money { return 1 })
		for _, size := range []int{0, -3} {
			_, err := orders.ImportProducts(t.Context(), db, slices.Values(rows), size)
			if !errors.Is(err, orders.ErrInvalidInput) {
				t.Errorf("batchSize %d error = %v, want ErrInvalidInput", size, err)
			}
		}
	})

	t.Run("bad row keeps committed batches and rolls back its own batch", func(t *testing.T) {
		db := migratedDB(t)
		rows := productRows(6, func(int) orders.Money { return 100 })
		rows[3].Price = -1 // row 4 is in the second batch [3,4]
		n, err := orders.ImportProducts(t.Context(), db, slices.Values(rows), 2)
		if !errors.Is(err, orders.ErrInvalidInput) {
			t.Errorf("error = %v, want ErrInvalidInput", err)
		}
		if n != 2 {
			t.Errorf("processed = %d, want 2 (first batch only)", n)
		}
		repo := orders.NewProductRepo(db)
		for i, r := range rows {
			_, err := repo.Get(t.Context(), r.SKU)
			switch {
			case i < 2 && err != nil:
				t.Errorf("row %d (committed batch) missing: %v", i, err)
			case i >= 2 && !errors.Is(err, orders.ErrNotFound):
				t.Errorf("row %d should not be present, got err=%v", i, err)
			}
		}
	})

	t.Run("input is consumed lazily in committed batches", func(t *testing.T) {
		db := migratedDB(t)
		rows := productRows(10, func(int) orders.Money { return 1 })
		committedBeforePull := -1
		seq := func(yield func(orders.Product) bool) {
			for i, r := range rows {
				if i == 4 {
					if err := db.QueryRow("SELECT count(*) FROM products").Scan(&committedBeforePull); err != nil {
						t.Errorf("count inside iterator: %v", err)
					}
				}
				if !yield(r) {
					return
				}
			}
		}
		var s iter.Seq[orders.Product] = seq
		n, err := orders.ImportProducts(t.Context(), db, s, 2)
		if err != nil || n != len(rows) {
			t.Fatalf("ImportProducts = %d, %v; want %d, nil", n, err, len(rows))
		}
		if committedBeforePull < 4 {
			t.Errorf("rows committed before pulling item 4 = %d, want at least 4 (batches must not accumulate)", committedBeforePull)
		}
	})

	t.Run("large import", func(t *testing.T) {
		db := migratedDB(t)
		rows := productRows(5000, func(i int) orders.Money { return orders.Money(i) })
		n, err := orders.ImportProducts(t.Context(), db, slices.Values(rows), 500)
		if err != nil || n != len(rows) {
			t.Fatalf("ImportProducts = %d, %v; want %d, nil", n, err, len(rows))
		}
		got, err := orders.NewProductRepo(db).Get(t.Context(), rows[4999].SKU)
		if err != nil || got.Price != 4999 {
			t.Errorf("last row = %+v, %v; want price 4999", got, err)
		}
	})

	t.Run("cancelled context stops the import", func(t *testing.T) {
		db := migratedDB(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		rows := productRows(3, func(int) orders.Money { return 1 })
		_, err := orders.ImportProducts(ctx, db, slices.Values(rows), 2)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("error = %v, want context.Canceled", err)
		}
	})
}
