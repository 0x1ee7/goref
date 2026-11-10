package structs_test

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	structs "goref/exercises/07_structs_and_methods"
)

// mustProduct builds a Product with NewProduct and fails the test on error.
func mustProduct(t *testing.T, sku, name string, price structs.Money, qty int) structs.Product {
	t.Helper()
	p, err := structs.NewProduct(sku, name, price, qty)
	if err != nil {
		t.Fatalf("NewProduct(%q): unexpected error: %v", sku, err)
	}
	return p
}

// mustCart returns a USD cart holding the given (product, quantity) pairs.
func mustCart(t *testing.T, items ...any) *structs.Cart {
	t.Helper()
	c, err := structs.NewCart("USD")
	if err != nil {
		t.Fatalf("NewCart: %v", err)
	}
	for i := 0; i < len(items); i += 2 {
		if err := c.Add(items[i].(structs.Product), items[i+1].(int)); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	return c
}

// lines collects the iterator into a slice of "sku=qty" strings.
func lines(c *structs.Cart) []string {
	var out []string
	for sku, qty := range c.All() {
		out = append(out, fmt.Sprintf("%s=%d", sku, qty))
	}
	return out
}

func TestPart2Product(t *testing.T) {
	t.Run("valid product trims name", func(t *testing.T) {
		p := mustProduct(t, "ABC-1234", "  Widget  ", mustMoney(t, 150, "USD"), 2)
		if p.SKU() != "ABC-1234" || p.Name() != "Widget" || p.Quantity() != 2 || !p.InStock() {
			t.Errorf("got sku=%q name=%q qty=%d inStock=%v", p.SKU(), p.Name(), p.Quantity(), p.InStock())
		}
		if p.Price() != mustMoney(t, 150, "USD") {
			t.Errorf("Price = %v, want 1.50 USD", p.Price())
		}
	})

	t.Run("zero quantity is valid but not in stock", func(t *testing.T) {
		p := mustProduct(t, "ABCD", "Gadget", mustMoney(t, 1, "USD"), 0)
		if p.InStock() {
			t.Errorf("InStock() = true with quantity 0")
		}
	})

	t.Run("invalid sku", func(t *testing.T) {
		for _, sku := range []string{"", "abc", "ABC", "ABCDEFGHIJKLMNOPQ", "ABC_1234", "ABC 123"} {
			t.Run(fmt.Sprintf("%q", sku), func(t *testing.T) {
				_ = mustMoney(t, 1, "USD")
				_, err := structs.NewProduct(sku, "Widget", mustMoney(t, 1, "USD"), 1)
				if !errors.Is(err, structs.ErrInvalidSKU) {
					t.Errorf("error = %v, want ErrInvalidSKU", err)
				}
			})
		}
	})

	t.Run("sku length bounds", func(t *testing.T) {
		for _, sku := range []string{"ABCD", "ABCDEFGHIJKLMNOP"} {
			t.Run(sku, func(t *testing.T) {
				p := mustProduct(t, sku, "Widget", mustMoney(t, 1, "USD"), 1)
				if p.SKU() != sku {
					t.Errorf("SKU() = %q, want %q", p.SKU(), sku)
				}
			})
		}
	})

	t.Run("invalid name", func(t *testing.T) {
		for _, name := range []string{"", "   ", strings.Repeat("x", 101)} {
			t.Run(fmt.Sprintf("len %d", len(name)), func(t *testing.T) {
				_ = mustMoney(t, 1, "USD")
				_, err := structs.NewProduct("ABCD", name, mustMoney(t, 1, "USD"), 1)
				if !errors.Is(err, structs.ErrInvalidName) {
					t.Errorf("error = %v, want ErrInvalidName", err)
				}
			})
		}
	})

	t.Run("name of 100 runes is valid", func(t *testing.T) {
		name := strings.Repeat("é", 100)
		p := mustProduct(t, "ABCD", name, mustMoney(t, 1, "USD"), 1)
		if p.Name() != name {
			t.Errorf("Name() length %d, want 100 runes", len([]rune(p.Name())))
		}
	})

	t.Run("invalid price and quantity", func(t *testing.T) {
		_ = mustMoney(t, 1, "USD")
		var zero structs.Money
		if _, err := structs.NewProduct("ABCD", "Widget", zero, 1); !errors.Is(err, structs.ErrInvalidCurrency) {
			t.Errorf("zero price error = %v, want ErrInvalidCurrency", err)
		}
		if _, err := structs.NewProduct("ABCD", "Widget", mustMoney(t, 1, "USD"), -1); !errors.Is(err, structs.ErrInvalidQuantity) {
			t.Errorf("negative quantity error = %v, want ErrInvalidQuantity", err)
		}
	})

	t.Run("restock returns a copy", func(t *testing.T) {
		p := mustProduct(t, "ABCD", "Widget", mustMoney(t, 1, "USD"), 2)
		more, err := p.Restock(3)
		if err != nil {
			t.Fatalf("Restock: %v", err)
		}
		if more.Quantity() != 5 {
			t.Errorf("restocked quantity = %d, want 5", more.Quantity())
		}
		if p.Quantity() != 2 {
			t.Errorf("original quantity = %d after Restock, want 2 (value semantics)", p.Quantity())
		}
	})

	t.Run("restock rejects bad counts", func(t *testing.T) {
		p := mustProduct(t, "ABCD", "Widget", mustMoney(t, 1, "USD"), 2)
		if _, err := p.Restock(0); !errors.Is(err, structs.ErrInvalidQuantity) {
			t.Errorf("Restock(0) error = %v, want ErrInvalidQuantity", err)
		}
		full := mustProduct(t, "ABCD", "Widget", mustMoney(t, 1, "USD"), math.MaxInt)
		if _, err := full.Restock(1); !errors.Is(err, structs.ErrInvalidQuantity) {
			t.Errorf("Restock past MaxInt error = %v, want ErrInvalidQuantity", err)
		}
	})
}

func TestPart2CartZeroValue(t *testing.T) {
	t.Run("zero cart adopts first product currency", func(t *testing.T) {
		_ = mustMoney(t, 1, "USD")
		var c structs.Cart
		if err := c.Add(mustProduct(t, "ABCD", "Widget", mustMoney(t, 150, "USD"), 1), 2); err != nil {
			t.Fatalf("Add: %v", err)
		}
		if c.Currency() != "USD" || c.Len() != 2 {
			t.Errorf("currency %q len %d, want USD and 2", c.Currency(), c.Len())
		}
	})

	t.Run("zero cart total before any add", func(t *testing.T) {
		_ = mustMoney(t, 1, "USD")
		var c structs.Cart
		total, err := c.Total()
		if err != nil {
			t.Fatalf("Total: %v", err)
		}
		if !total.IsZero() || c.Len() != 0 {
			t.Errorf("empty zero cart total %v, len %d", total, c.Len())
		}
	})

	t.Run("zero cart rejects product in another currency", func(t *testing.T) {
		_ = mustMoney(t, 1, "USD")
		var c structs.Cart
		if err := c.Add(mustProduct(t, "ABCD", "Widget", mustMoney(t, 150, "USD"), 1), 1); err != nil {
			t.Fatalf("first Add: %v", err)
		}
		eur := mustProduct(t, "EFGH", "Gizmo", mustMoney(t, 150, "EUR"), 1)
		if err := c.Add(eur, 1); !errors.Is(err, structs.ErrCurrencyMismatch) {
			t.Errorf("error = %v, want ErrCurrencyMismatch", err)
		}
	})
}

func TestPart2CartConstructor(t *testing.T) {
	t.Run("invalid currency", func(t *testing.T) {
		_ = mustMoney(t, 1, "USD")
		if _, err := structs.NewCart("usd"); !errors.Is(err, structs.ErrInvalidCurrency) {
			t.Errorf("error = %v, want ErrInvalidCurrency", err)
		}
	})

	t.Run("fixed currency empty total", func(t *testing.T) {
		c, err := structs.NewCart("USD")
		if err != nil {
			t.Fatalf("NewCart: %v", err)
		}
		total, err := c.Total()
		if err != nil {
			t.Fatalf("Total: %v", err)
		}
		if c.Currency() != "USD" || total.Currency() != "USD" || !total.IsZero() {
			t.Errorf("currency %q, total %v", c.Currency(), total)
		}
	})

	t.Run("product in other currency rejected", func(t *testing.T) {
		c, err := structs.NewCart("USD")
		if err != nil {
			t.Fatalf("NewCart: %v", err)
		}
		eur := mustProduct(t, "ABCD", "Widget", mustMoney(t, 150, "EUR"), 1)
		if err := c.Add(eur, 1); !errors.Is(err, structs.ErrCurrencyMismatch) {
			t.Errorf("error = %v, want ErrCurrencyMismatch", err)
		}
	})

	t.Run("invalid add arguments", func(t *testing.T) {
		c := mustCart(t)
		p := mustProduct(t, "ABCD", "Widget", mustMoney(t, 1, "USD"), 1)
		if err := c.Add(p, 0); !errors.Is(err, structs.ErrInvalidQuantity) {
			t.Errorf("Add qty 0 error = %v, want ErrInvalidQuantity", err)
		}
		if err := c.Add(p, -2); !errors.Is(err, structs.ErrInvalidQuantity) {
			t.Errorf("Add qty -2 error = %v, want ErrInvalidQuantity", err)
		}
		var zeroProduct structs.Product
		if err := c.Add(zeroProduct, 1); !errors.Is(err, structs.ErrInvalidSKU) {
			t.Errorf("Add zero Product error = %v, want ErrInvalidSKU", err)
		}
	})
}

func TestPart2CartAddRemove(t *testing.T) {
	widget := mustProduct(t, "WIDG-1", "Widget", mustMoney(t, 150, "USD"), 10)
	gadget := mustProduct(t, "GADG-1", "Gadget", mustMoney(t, 1000, "USD"), 10)

	t.Run("add accumulates same sku", func(t *testing.T) {
		c := mustCart(t, widget, 2)
		if err := c.Add(widget, 3); err != nil {
			t.Fatalf("Add: %v", err)
		}
		if got := c.Quantity("WIDG-1"); got != 5 {
			t.Errorf("Quantity = %d, want 5", got)
		}
		if diff := cmp.Diff([]string{"WIDG-1=5"}, lines(c)); diff != "" {
			t.Errorf("lines mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("iteration keeps insertion order", func(t *testing.T) {
		c := mustCart(t, widget, 1, gadget, 2, widget, 2)
		want := []string{"WIDG-1=3", "GADG-1=2"}
		if diff := cmp.Diff(want, lines(c)); diff != "" {
			t.Errorf("lines mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("partial remove", func(t *testing.T) {
		c := mustCart(t, widget, 5)
		if err := c.Remove("WIDG-1", 2); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if c.Quantity("WIDG-1") != 3 || c.Len() != 3 {
			t.Errorf("quantity %d len %d, want 3 and 3", c.Quantity("WIDG-1"), c.Len())
		}
	})

	t.Run("removing all deletes the line and keeps order", func(t *testing.T) {
		c := mustCart(t, widget, 1, gadget, 1, mustProduct(t, "THNG-1", "Thing", mustMoney(t, 5, "USD"), 1), 1)
		if err := c.Remove("GADG-1", 1); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if c.Has("GADG-1") {
			t.Errorf("Has(GADG-1) = true after removing last unit")
		}
		want := []string{"WIDG-1=1", "THNG-1=1"}
		if diff := cmp.Diff(want, lines(c)); diff != "" {
			t.Errorf("lines mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("remove unknown sku", func(t *testing.T) {
		c := mustCart(t, widget, 1)
		if err := c.Remove("NOPE-1", 1); !errors.Is(err, structs.ErrNotInCart) {
			t.Errorf("error = %v, want ErrNotInCart", err)
		}
	})

	t.Run("remove more than held leaves cart unchanged", func(t *testing.T) {
		c := mustCart(t, widget, 2)
		if err := c.Remove("WIDG-1", 3); !errors.Is(err, structs.ErrInsufficientQuantity) {
			t.Errorf("error = %v, want ErrInsufficientQuantity", err)
		}
		if c.Quantity("WIDG-1") != 2 {
			t.Errorf("quantity = %d after failed remove, want 2", c.Quantity("WIDG-1"))
		}
	})

	t.Run("remove with non-positive quantity", func(t *testing.T) {
		c := mustCart(t, widget, 2)
		if err := c.Remove("WIDG-1", 0); !errors.Is(err, structs.ErrInvalidQuantity) {
			t.Errorf("error = %v, want ErrInvalidQuantity", err)
		}
	})
}

func TestPart2CartQueries(t *testing.T) {
	widget := mustProduct(t, "WIDG-1", "Widget", mustMoney(t, 150, "USD"), 10)

	t.Run("quantity and has", func(t *testing.T) {
		c := mustCart(t, widget, 4)
		if c.Quantity("WIDG-1") != 4 || !c.Has("WIDG-1") {
			t.Errorf("Quantity = %d, Has = %v", c.Quantity("WIDG-1"), c.Has("WIDG-1"))
		}
		if c.Quantity("NOPE-1") != 0 || c.Has("NOPE-1") {
			t.Errorf("absent sku: Quantity = %d, Has = %v; want 0, false", c.Quantity("NOPE-1"), c.Has("NOPE-1"))
		}
	})

	t.Run("len sums units", func(t *testing.T) {
		gadget := mustProduct(t, "GADG-1", "Gadget", mustMoney(t, 1000, "USD"), 10)
		c := mustCart(t, widget, 2, gadget, 3)
		if c.Len() != 5 {
			t.Errorf("Len = %d, want 5", c.Len())
		}
	})

	t.Run("iteration can break early", func(t *testing.T) {
		gadget := mustProduct(t, "GADG-1", "Gadget", mustMoney(t, 1000, "USD"), 10)
		c := mustCart(t, widget, 2, gadget, 3)
		seen := 0
		for range c.All() {
			seen++
			break
		}
		if seen != 1 {
			t.Errorf("loop body ran %d times after break, want 1", seen)
		}
	})

	t.Run("string summary", func(t *testing.T) {
		c := mustCart(t, widget, 3)
		if got, want := c.String(), `Cart(currency="USD", items=3)`; got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	})

	t.Run("pointer receiver String", func(t *testing.T) {
		c := mustCart(t, widget, 1)
		if got := fmt.Sprint(*c); strings.HasPrefix(got, "Cart(") {
			t.Errorf("value Cart used pointer String method: %q", got)
		}
		if got := fmt.Sprint(c); !strings.HasPrefix(got, "Cart(") {
			t.Errorf("pointer Cart did not use String: %q", got)
		}
	})
}

func TestPart2CartTotal(t *testing.T) {
	t.Run("sums price times quantity", func(t *testing.T) {
		widget := mustProduct(t, "WIDG-1", "Widget", mustMoney(t, 150, "USD"), 10)
		gadget := mustProduct(t, "GADG-1", "Gadget", mustMoney(t, 1000, "USD"), 10)
		c := mustCart(t, widget, 2, gadget, 1)
		total, err := c.Total()
		if err != nil {
			t.Fatalf("Total: %v", err)
		}
		if total != mustMoney(t, 1300, "USD") {
			t.Errorf("Total = %v, want 13.00 USD", total)
		}
	})

	t.Run("overflow", func(t *testing.T) {
		huge := mustProduct(t, "HUGE-1", "Huge", mustMoney(t, math.MaxInt64, "USD"), 10)
		c := mustCart(t, huge, 2)
		if _, err := c.Total(); !errors.Is(err, structs.ErrOverflow) {
			t.Errorf("Total error = %v, want ErrOverflow", err)
		}
	})
}
