package orders_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	orders "goref/exercises/13_database_sql"

	"github.com/google/go-cmp/cmp"
)

func TestPart4Customers(t *testing.T) {
	t.Run("add returns the stored row in UTC with microseconds", func(t *testing.T) {
		db := migratedDB(t)
		repo := orders.NewCustomerRepo(db)
		phone := sql.Null[string]{V: "+1-555-0100", Valid: true}
		got, err := repo.Add(t.Context(), orders.NewCustomer{
			Email:     "ann@example.com",
			Name:      "Ann Lee",
			Phone:     phone,
			CreatedAt: mustTime(t, "2026-01-02T03:04:05.123456789+01:00"),
		})
		if err != nil {
			t.Fatalf("Add: %v", err)
		}
		if got.ID <= 0 {
			t.Errorf("ID = %d, want a generated positive id", got.ID)
		}
		want := orders.Customer{
			ID:        got.ID,
			Email:     "ann@example.com",
			Name:      "Ann Lee",
			Phone:     phone,
			CreatedAt: time.Date(2026, 1, 2, 2, 4, 5, 123456000, time.UTC),
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Add() mismatch (-want +got):\n%s", diff)
		}
		if loc := got.CreatedAt.Location(); loc != time.UTC {
			t.Errorf("CreatedAt location = %v, want UTC", loc)
		}
	})

	t.Run("missing phone stays NULL", func(t *testing.T) {
		db := migratedDB(t)
		c := addCustomer(t, db, "bob@example.com", "Bob")
		got, err := orders.NewCustomerRepo(db).Get(t.Context(), c.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Phone.Valid {
			t.Errorf("Phone = %+v, want NULL", got.Phone)
		}
	})

	t.Run("duplicate email is rejected in any case", func(t *testing.T) {
		db := migratedDB(t)
		addCustomer(t, db, "ann@example.com", "Ann")
		_, err := orders.NewCustomerRepo(db).Add(t.Context(), orders.NewCustomer{
			Email:     "ANN@example.com",
			Name:      "Impostor",
			CreatedAt: mustTime(t, "2026-01-01T00:00:00Z"),
		})
		if !errors.Is(err, orders.ErrDuplicateEmail) {
			t.Errorf("Add duplicate error = %v, want ErrDuplicateEmail", err)
		}
	})

	t.Run("get by id", func(t *testing.T) {
		db := migratedDB(t)
		c := addCustomer(t, db, "ann@example.com", "Ann")
		repo := orders.NewCustomerRepo(db)
		got, err := repo.Get(t.Context(), c.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if diff := cmp.Diff(c, got); diff != "" {
			t.Errorf("Get() mismatch (-want +got):\n%s", diff)
		}
		if _, err := repo.Get(t.Context(), 999); !errors.Is(err, orders.ErrNotFound) {
			t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
		}
	})

	t.Run("get by email is case-insensitive", func(t *testing.T) {
		db := migratedDB(t)
		c := addCustomer(t, db, "ann@example.com", "Ann")
		repo := orders.NewCustomerRepo(db)
		got, err := repo.GetByEmail(t.Context(), "ANN@Example.COM")
		if err != nil {
			t.Fatalf("GetByEmail: %v", err)
		}
		if got.ID != c.ID {
			t.Errorf("GetByEmail ID = %d, want %d", got.ID, c.ID)
		}
		if _, err := repo.GetByEmail(t.Context(), "nobody@example.com"); !errors.Is(err, orders.ErrNotFound) {
			t.Errorf("GetByEmail(missing) error = %v, want ErrNotFound", err)
		}
	})

	t.Run("search is literal, case-insensitive and ordered by id", func(t *testing.T) {
		db := migratedDB(t)
		addCustomer(t, db, "a@example.com", "Ann Lee")
		addCustomer(t, db, "b@example.com", "Bob Stone")
		addCustomer(t, db, "c@example.com", "Joanna 100% Organic")
		addCustomer(t, db, "d@example.com", "Ann_Under")
		addCustomer(t, db, "e@example.com", "Zed")
		repo := orders.NewCustomerRepo(db)
		for _, tc := range []struct {
			term string
			want []string
		}{
			{"ann", []string{"Ann Lee", "Joanna 100% Organic", "Ann_Under"}},
			{"%", []string{"Joanna 100% Organic"}},
			{"_", []string{"Ann_Under"}},
			{"no-such-name", nil},
		} {
			got, err := repo.SearchByName(t.Context(), tc.term)
			if err != nil {
				t.Errorf("SearchByName(%q): %v", tc.term, err)
				continue
			}
			var names []string
			for _, c := range got {
				names = append(names, c.Name)
			}
			if diff := cmp.Diff(tc.want, names); diff != "" {
				t.Errorf("SearchByName(%q) mismatch (-want +got):\n%s", tc.term, diff)
			}
		}
	})

	t.Run("empty search term is invalid", func(t *testing.T) {
		db := migratedDB(t)
		if _, err := orders.NewCustomerRepo(db).SearchByName(t.Context(), ""); !errors.Is(err, orders.ErrInvalidInput) {
			t.Errorf("SearchByName(\"\") error = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("hostile input is inert", func(t *testing.T) {
		db := migratedDB(t)
		addCustomer(t, db, "ann@example.com", "Ann")
		evil := "x'); DROP TABLE customers; --"
		repo := orders.NewCustomerRepo(db)
		if _, err := repo.Add(t.Context(), orders.NewCustomer{
			Email:     "evil@example.com",
			Name:      evil,
			CreatedAt: mustTime(t, "2026-01-01T00:00:00Z"),
		}); err != nil {
			t.Fatalf("Add hostile name: %v", err)
		}
		got, err := repo.SearchByName(t.Context(), evil)
		if err != nil {
			t.Fatalf("SearchByName: %v", err)
		}
		if len(got) != 1 || got[0].Name != evil {
			t.Errorf("SearchByName(hostile) = %+v, want exactly the hostile row", got)
		}
		if _, err := repo.GetByEmail(t.Context(), "ann@example.com"); err != nil {
			t.Errorf("table damaged by injected input: %v", err)
		}
	})

	t.Run("cancelled context is honoured", func(t *testing.T) {
		db := migratedDB(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := orders.NewCustomerRepo(db).Add(ctx, orders.NewCustomer{
			Email:     "ann@example.com",
			Name:      "Ann",
			CreatedAt: mustTime(t, "2026-01-01T00:00:00Z"),
		})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Add with cancelled ctx error = %v, want context.Canceled", err)
		}
	})
}

func TestPart4Products(t *testing.T) {
	t.Run("upsert inserts then replaces name and price", func(t *testing.T) {
		db := migratedDB(t)
		repo := orders.NewProductRepo(db)
		ctx := t.Context()
		if err := repo.Upsert(ctx, orders.Product{SKU: "BOOK-1", Name: "Go in Practice", Price: 4999}); err != nil {
			t.Fatalf("first Upsert: %v", err)
		}
		if err := repo.Upsert(ctx, orders.Product{SKU: "BOOK-1", Name: "Go in Practice, 2nd ed.", Price: 5999}); err != nil {
			t.Fatalf("second Upsert: %v", err)
		}
		got, err := repo.Get(ctx, "BOOK-1")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		want := orders.Product{SKU: "BOOK-1", Name: "Go in Practice, 2nd ed.", Price: 5999}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Get() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("negative price is rejected and nothing is written", func(t *testing.T) {
		db := migratedDB(t)
		repo := orders.NewProductRepo(db)
		if err := repo.Upsert(t.Context(), orders.Product{SKU: "BAD", Name: "Bad", Price: -1}); !errors.Is(err, orders.ErrInvalidInput) {
			t.Errorf("Upsert(negative) error = %v, want ErrInvalidInput", err)
		}
		if _, err := repo.Get(t.Context(), "BAD"); !errors.Is(err, orders.ErrNotFound) {
			t.Errorf("Get(BAD) error = %v, want ErrNotFound", err)
		}
	})

	t.Run("zero price is allowed", func(t *testing.T) {
		db := migratedDB(t)
		repo := orders.NewProductRepo(db)
		if err := repo.Upsert(t.Context(), orders.Product{SKU: "FREE", Name: "Sticker", Price: 0}); err != nil {
			t.Fatalf("Upsert(0): %v", err)
		}
		got, err := repo.Get(t.Context(), "FREE")
		if err != nil || got.Price != 0 {
			t.Errorf("Get(FREE) = %+v, %v; want price 0", got, err)
		}
	})

	t.Run("get missing SKU is ErrNotFound", func(t *testing.T) {
		db := migratedDB(t)
		if _, err := orders.NewProductRepo(db).Get(t.Context(), "NOPE"); !errors.Is(err, orders.ErrNotFound) {
			t.Errorf("Get(NOPE) error = %v, want ErrNotFound", err)
		}
	})
}

func TestPart4Orders(t *testing.T) {
	t.Run("place snapshots catalogue prices", func(t *testing.T) {
		db := migratedDB(t)
		seedCatalogue(t, db)
		c := addCustomer(t, db, "ann@example.com", "Ann")
		repo := orders.NewOrderRepo(db)
		placed := mustTime(t, "2026-03-01T10:00:00Z")
		o, err := repo.Place(t.Context(), c.ID, placed, []orders.Line{
			{SKU: "BOOK-1", Quantity: 2},
			{SKU: "MUG-7", Quantity: 1},
		})
		if err != nil {
			t.Fatalf("Place: %v", err)
		}
		wantOrder := orders.Order{ID: o.ID, CustomerID: c.ID, PlacedAt: placed, Status: orders.StatusPending}
		if diff := cmp.Diff(wantOrder, o); diff != "" {
			t.Errorf("Place() mismatch (-want +got):\n%s", diff)
		}
		wantItems := []orders.OrderItem{
			{OrderID: o.ID, LineNo: 1, SKU: "BOOK-1", Quantity: 2, UnitPrice: 4999},
			{OrderID: o.ID, LineNo: 2, SKU: "MUG-7", Quantity: 1, UnitPrice: 1250},
		}
		if err := orders.NewProductRepo(db).Upsert(t.Context(), orders.Product{SKU: "BOOK-1", Name: "Go in Practice", Price: 9999}); err != nil {
			t.Fatalf("reprice: %v", err)
		}
		items, err := repo.Items(t.Context(), o.ID)
		if err != nil {
			t.Fatalf("Items: %v", err)
		}
		if diff := cmp.Diff(wantItems, items); diff != "" {
			t.Errorf("Items() after repricing mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("line numbers follow input order", func(t *testing.T) {
		db := migratedDB(t)
		seedCatalogue(t, db)
		c := addCustomer(t, db, "ann@example.com", "Ann")
		o := placeAt(t, db, c.ID, "2026-03-01T10:00:00Z",
			orders.Line{SKU: "MUG-7", Quantity: 1},
			orders.Line{SKU: "BOOK-1", Quantity: 1},
			orders.Line{SKU: "MUG-7", Quantity: 3},
		)
		items, err := orders.NewOrderRepo(db).Items(t.Context(), o.ID)
		if err != nil {
			t.Fatalf("Items: %v", err)
		}
		var got []string
		var lines []int
		for _, it := range items {
			got = append(got, it.SKU)
			lines = append(lines, it.LineNo)
		}
		if diff := cmp.Diff([]string{"MUG-7", "BOOK-1", "MUG-7"}, got); diff != "" {
			t.Errorf("SKUs mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff([]int{1, 2, 3}, lines); diff != "" {
			t.Errorf("LineNo mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("placed_at is stored as UTC microseconds", func(t *testing.T) {
		db := migratedDB(t)
		seedCatalogue(t, db)
		c := addCustomer(t, db, "ann@example.com", "Ann")
		o := placeAt(t, db, c.ID, "2026-03-01T12:00:00.000000789+02:00", orders.Line{SKU: "MUG-7", Quantity: 1})
		want := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
		if !o.PlacedAt.Equal(want) || o.PlacedAt.Location() != time.UTC {
			t.Errorf("PlacedAt = %v (%v), want %v in UTC", o.PlacedAt, o.PlacedAt.Location(), want)
		}
	})

	t.Run("empty lines are invalid", func(t *testing.T) {
		db := migratedDB(t)
		seedCatalogue(t, db)
		c := addCustomer(t, db, "ann@example.com", "Ann")
		_, err := orders.NewOrderRepo(db).Place(t.Context(), c.ID, mustTime(t, "2026-03-01T10:00:00Z"), nil)
		if !errors.Is(err, orders.ErrInvalidInput) {
			t.Errorf("Place(no lines) error = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("quantity below one is invalid", func(t *testing.T) {
		db := migratedDB(t)
		seedCatalogue(t, db)
		c := addCustomer(t, db, "ann@example.com", "Ann")
		repo := orders.NewOrderRepo(db)
		for _, q := range []int{0, -1} {
			_, err := repo.Place(t.Context(), c.ID, mustTime(t, "2026-03-01T10:00:00Z"),
				[]orders.Line{{SKU: "MUG-7", Quantity: q}})
			if !errors.Is(err, orders.ErrInvalidInput) {
				t.Errorf("Place(quantity %d) error = %v, want ErrInvalidInput", q, err)
			}
		}
	})

	t.Run("unknown SKUs are reported sorted and unique", func(t *testing.T) {
		db := migratedDB(t)
		seedCatalogue(t, db)
		c := addCustomer(t, db, "ann@example.com", "Ann")
		_, err := orders.NewOrderRepo(db).Place(t.Context(), c.ID, mustTime(t, "2026-03-01T10:00:00Z"), []orders.Line{
			{SKU: "ZZZ", Quantity: 1},
			{SKU: "BOOK-1", Quantity: 1},
			{SKU: "AAA", Quantity: 1},
			{SKU: "ZZZ", Quantity: 2},
		})
		var uerr *orders.UnknownSKUError
		if !errors.As(err, &uerr) {
			t.Fatalf("Place error = %v, want *UnknownSKUError", err)
		}
		if diff := cmp.Diff([]string{"AAA", "ZZZ"}, uerr.SKUs); diff != "" {
			t.Errorf("UnknownSKUError.SKUs mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("unknown customer is ErrNotFound and leaves no order behind", func(t *testing.T) {
		db := migratedDB(t)
		seedCatalogue(t, db)
		repo := orders.NewOrderRepo(db)
		_, err := repo.Place(t.Context(), 9999, mustTime(t, "2026-03-01T10:00:00Z"),
			[]orders.Line{{SKU: "MUG-7", Quantity: 1}})
		if !errors.Is(err, orders.ErrNotFound) {
			t.Errorf("Place(unknown customer) error = %v, want ErrNotFound", err)
		}
		if _, err := repo.Get(t.Context(), 1); !errors.Is(err, orders.ErrNotFound) {
			t.Errorf("Get(1) after failed Place error = %v, want ErrNotFound", err)
		}
	})

	t.Run("failed place with unknown SKU leaves no order header", func(t *testing.T) {
		db := migratedDB(t)
		seedCatalogue(t, db)
		c := addCustomer(t, db, "ann@example.com", "Ann")
		repo := orders.NewOrderRepo(db)
		if _, err := repo.Place(t.Context(), c.ID, mustTime(t, "2026-03-01T10:00:00Z"),
			[]orders.Line{{SKU: "NOPE", Quantity: 1}}); err == nil {
			t.Fatal("Place with unknown SKU succeeded")
		}
		if _, err := repo.Get(t.Context(), 1); !errors.Is(err, orders.ErrNotFound) {
			t.Errorf("Get(1) after failed Place error = %v, want ErrNotFound", err)
		}
	})

	t.Run("set status accepts the four statuses and rejects others", func(t *testing.T) {
		db := migratedDB(t)
		seedCatalogue(t, db)
		c := addCustomer(t, db, "ann@example.com", "Ann")
		o := placeAt(t, db, c.ID, "2026-03-01T10:00:00Z", orders.Line{SKU: "MUG-7", Quantity: 1})
		repo := orders.NewOrderRepo(db)
		for _, s := range []orders.Status{orders.StatusPaid, orders.StatusShipped, orders.StatusCancelled, orders.StatusPending} {
			if err := repo.SetStatus(t.Context(), o.ID, s); err != nil {
				t.Fatalf("SetStatus(%s): %v", s, err)
			}
			got, err := repo.Get(t.Context(), o.ID)
			if err != nil || got.Status != s {
				t.Errorf("after SetStatus(%s): status = %q, %v", s, got.Status, err)
			}
		}
		if err := repo.SetStatus(t.Context(), o.ID, "refunded"); !errors.Is(err, orders.ErrInvalidInput) {
			t.Errorf("SetStatus(refunded) error = %v, want ErrInvalidInput", err)
		}
		if err := repo.SetStatus(t.Context(), 999, orders.StatusPaid); !errors.Is(err, orders.ErrNotFound) {
			t.Errorf("SetStatus(unknown id) error = %v, want ErrNotFound", err)
		}
	})

	t.Run("items of an unknown order are empty", func(t *testing.T) {
		db := migratedDB(t)
		items, err := orders.NewOrderRepo(db).Items(t.Context(), 999)
		if err != nil || len(items) != 0 {
			t.Errorf("Items(999) = %v, %v; want empty, nil", items, err)
		}
	})
}
