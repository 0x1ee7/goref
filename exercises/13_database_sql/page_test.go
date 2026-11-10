package orders_test

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"math/rand/v2"
	"strings"
	"testing"

	orders "goref/exercises/13_database_sql"

	"github.com/google/go-cmp/cmp"
)

// seedPaging places five orders whose (placed_at, id) order is [2, 3, 1, 4, 5]:
// ids 2 and 3 share 10:00, ids 4 and 5 share 10:10.
func seedPaging(t *testing.T, db *sql.DB) []int64 {
	t.Helper()
	seedCatalogue(t, db)
	c := addCustomer(t, db, "ann@example.com", "Ann")
	times := []string{
		"2026-04-01T10:05:00Z",
		"2026-04-01T10:00:00Z",
		"2026-04-01T10:00:00Z",
		"2026-04-01T10:10:00Z",
		"2026-04-01T10:10:00Z",
	}
	ids := make([]int64, len(times))
	for i, at := range times {
		ids[i] = placeAt(t, db, c.ID, at, orders.Line{SKU: "MUG-7", Quantity: 1}).ID
	}
	return ids
}

func idsOf(list []orders.Order) []int64 {
	out := make([]int64, 0, len(list))
	for _, o := range list {
		out = append(out, o.ID)
	}
	return out
}

func TestPart6Cursor(t *testing.T) {
	t.Run("round trip", func(t *testing.T) {
		c := orders.Cursor{PlacedAt: mustTime(t, "2026-04-01T10:05:00.000123Z"), ID: 42}
		got, err := orders.DecodeCursor(c.Encode())
		if err != nil {
			t.Fatalf("DecodeCursor(Encode()): %v", err)
		}
		if diff := cmp.Diff(c, got); diff != "" {
			t.Errorf("round trip mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("token is URL-safe and unpadded", func(t *testing.T) {
		// Pick an id that forces base64 characters beyond the alphanumerics.
		for id := int64(1); id < 200; id++ {
			c := orders.Cursor{PlacedAt: mustTime(t, "2026-04-01T10:05:00Z"), ID: id}
			tok := c.Encode()
			if strings.ContainsAny(tok, "+/= \t\n") {
				t.Fatalf("token %q for id %d is not URL-safe and unpadded", tok, id)
			}
			got, err := orders.DecodeCursor(tok)
			if err != nil || got.ID != id || !got.PlacedAt.Equal(c.PlacedAt) {
				t.Fatalf("token %q for id %d does not round-trip: got %+v, err %v", tok, id, got, err)
			}
		}
	})

	t.Run("decode rejects malformed input", func(t *testing.T) {
		enc := base64.RawURLEncoding.EncodeToString
		valid := orders.Cursor{PlacedAt: mustTime(t, "2026-04-01T10:05:00Z"), ID: 7}.Encode()
		for name, tok := range map[string]string{
			"empty":            "",
			"bad base64":       "!!!",
			"padded":           valid + "=",
			"no separator":     enc([]byte("1743501900000000")),
			"non-integer time": enc([]byte("abc:7")),
			"non-integer id":   enc([]byte("1743501900000000:xyz")),
			"empty time":       enc([]byte(":7")),
			"empty id":         enc([]byte("1743501900000000:")),
		} {
			if _, err := orders.DecodeCursor(tok); !errors.Is(err, orders.ErrInvalidInput) {
				t.Errorf("%s: DecodeCursor(%q) error = %v, want ErrInvalidInput", name, tok, err)
			}
		}
	})

	t.Run("decode never panics on random input", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		valid := orders.Cursor{PlacedAt: mustTime(t, "2026-04-01T10:05:00Z"), ID: 7}.Encode()
		if _, err := orders.DecodeCursor(valid); err != nil {
			t.Fatalf("DecodeCursor(%q) of a valid token failed: %v", valid, err)
		}
		for range 2000 {
			buf := make([]byte, rng.IntN(40))
			for j := range buf {
				buf[j] = byte(rng.IntN(256))
			}
			for _, tok := range []string{
				base64.RawURLEncoding.EncodeToString(buf),
				string(buf),
			} {
				c, err := orders.DecodeCursor(tok)
				if err != nil {
					continue
				}
				back, err := orders.DecodeCursor(c.Encode())
				if err != nil || back.ID != c.ID || !back.PlacedAt.Equal(c.PlacedAt) {
					t.Fatalf("accepted token %q does not re-encode to an equal cursor: %+v -> %+v, err %v", tok, c, back, err)
				}
			}
		}
	})

	t.Run("valid cursors always decode", func(t *testing.T) {
		for id := int64(1); id < 200; id++ {
			c := orders.Cursor{PlacedAt: mustTime(t, "2026-04-01T10:05:00Z"), ID: id}
			if _, err := orders.DecodeCursor(c.Encode()); err != nil {
				t.Fatalf("DecodeCursor of a valid token failed: %v", err)
			}
		}
	})
}

func TestPart6PageOrders(t *testing.T) {
	t.Run("pages walk the full list in order without gaps", func(t *testing.T) {
		db := migratedDB(t)
		ids := seedPaging(t, db)
		repo := orders.NewOrderRepo(db)
		want := []int64{ids[1], ids[2], ids[0], ids[3], ids[4]}
		var got []int64
		token := ""
		for pages := 0; ; pages++ {
			if pages > 10 {
				t.Fatal("paging did not terminate")
			}
			page, err := repo.PageOrders(t.Context(), token, 2)
			if err != nil {
				t.Fatalf("PageOrders(%q, 2): %v", token, err)
			}
			got = append(got, idsOf(page.Orders)...)
			if page.NextToken == "" {
				break
			}
			token = page.NextToken
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("ids mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("full final page yields an empty trailing page", func(t *testing.T) {
		db := migratedDB(t)
		seedPaging(t, db)
		repo := orders.NewOrderRepo(db)
		page, err := repo.PageOrders(t.Context(), "", 5)
		if err != nil {
			t.Fatalf("PageOrders: %v", err)
		}
		if len(page.Orders) != 5 || page.NextToken == "" {
			t.Fatalf("full page = %d orders, next=%q; want 5 and a token", len(page.Orders), page.NextToken)
		}
		tail, err := repo.PageOrders(t.Context(), page.NextToken, 5)
		if err != nil {
			t.Fatalf("PageOrders(tail): %v", err)
		}
		if len(tail.Orders) != 0 || tail.NextToken != "" {
			t.Errorf("tail = %d orders, next=%q; want empty and no token", len(tail.Orders), tail.NextToken)
		}
	})

	t.Run("empty database yields an empty page", func(t *testing.T) {
		db := migratedDB(t)
		page, err := orders.NewOrderRepo(db).PageOrders(t.Context(), "", 3)
		if err != nil || len(page.Orders) != 0 || page.NextToken != "" {
			t.Errorf("PageOrders on empty db = %+v, %v; want empty page, nil", page, err)
		}
	})

	t.Run("invalid limit and token are rejected", func(t *testing.T) {
		db := migratedDB(t)
		repo := orders.NewOrderRepo(db)
		for _, limit := range []int{0, -1} {
			if _, err := repo.PageOrders(t.Context(), "", limit); !errors.Is(err, orders.ErrInvalidInput) {
				t.Errorf("PageOrders(limit %d) error = %v, want ErrInvalidInput", limit, err)
			}
		}
		if _, err := repo.PageOrders(t.Context(), "!!garbage!!", 2); !errors.Is(err, orders.ErrInvalidInput) {
			t.Errorf("PageOrders(garbage token) error = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("older rows inserted between calls do not overlap pages", func(t *testing.T) {
		db := migratedDB(t)
		seedPaging(t, db)
		repo := orders.NewOrderRepo(db)
		first, err := repo.PageOrders(t.Context(), "", 2)
		if err != nil {
			t.Fatalf("first page: %v", err)
		}
		c := addCustomer(t, db, "late@example.com", "Late")
		placeAt(t, db, c.ID, "2026-04-01T09:00:00Z", orders.Line{SKU: "MUG-7", Quantity: 1})
		second, err := repo.PageOrders(t.Context(), first.NextToken, 2)
		if err != nil {
			t.Fatalf("second page: %v", err)
		}
		if diff := cmp.Diff([]int64{1, 4}, idsOf(second.Orders)); diff != "" {
			t.Errorf("second page ids mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("offset pages shift under an insert", func(t *testing.T) {
		db := migratedDB(t)
		seedPaging(t, db)
		repo := orders.NewOrderRepo(db)
		first, err := repo.PageOrdersOffset(t.Context(), 0, 2)
		if err != nil {
			t.Fatalf("first offset page: %v", err)
		}
		if diff := cmp.Diff([]int64{2, 3}, idsOf(first)); diff != "" {
			t.Fatalf("first offset page mismatch (-want +got):\n%s", diff)
		}
		c := addCustomer(t, db, "late@example.com", "Late")
		placeAt(t, db, c.ID, "2026-04-01T09:00:00Z", orders.Line{SKU: "MUG-7", Quantity: 1})
		second, err := repo.PageOrdersOffset(t.Context(), 2, 2)
		if err != nil {
			t.Fatalf("second offset page: %v", err)
		}
		// The new older row shifts everything right, so id 3 is seen twice.
		if diff := cmp.Diff([]int64{3, 1}, idsOf(second)); diff != "" {
			t.Errorf("second offset page mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("offset past the end is empty", func(t *testing.T) {
		db := migratedDB(t)
		seedPaging(t, db)
		got, err := orders.NewOrderRepo(db).PageOrdersOffset(t.Context(), 100, 5)
		if err != nil || len(got) != 0 {
			t.Errorf("PageOrdersOffset(100) = %v, %v; want empty, nil", got, err)
		}
	})

	t.Run("negative offset and non-positive limit are invalid", func(t *testing.T) {
		db := migratedDB(t)
		repo := orders.NewOrderRepo(db)
		if _, err := repo.PageOrdersOffset(t.Context(), -1, 2); !errors.Is(err, orders.ErrInvalidInput) {
			t.Errorf("offset -1 error = %v, want ErrInvalidInput", err)
		}
		if _, err := repo.PageOrdersOffset(t.Context(), 0, 0); !errors.Is(err, orders.ErrInvalidInput) {
			t.Errorf("limit 0 error = %v, want ErrInvalidInput", err)
		}
	})
}
