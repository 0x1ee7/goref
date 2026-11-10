package structs

import (
	"errors"
	"iter"
)

var (
	// ErrInvalidSKU reports a SKU that is not 4-16 characters from A-Z, 0-9 and '-'.
	ErrInvalidSKU = errors.New("invalid sku")
	// ErrInvalidName reports an empty or over-long product name.
	ErrInvalidName = errors.New("invalid product name")
	// ErrInvalidQuantity reports a quantity that is out of range for the operation.
	ErrInvalidQuantity = errors.New("invalid quantity")
	// ErrNotInCart reports a SKU the cart does not hold.
	ErrNotInCart = errors.New("sku not in cart")
	// ErrInsufficientQuantity reports a removal larger than the quantity held.
	ErrInsufficientQuantity = errors.New("not enough units in cart")
)

// Product is an immutable catalogue entry. Use NewProduct; the zero value is not
// a valid product. Methods take value receivers because a Product is a small
// value that is copied freely.
type Product struct {
	sku      string
	name     string
	price    Money
	quantity int
}

// NewProduct validates and returns a product. The name is stored trimmed.
// quantity must be >= 0. The price currency must be valid.
func NewProduct(sku, name string, price Money, quantity int) (Product, error) {
	return Product{}, errNotImplemented
}

// SKU returns the stock-keeping unit.
func (p Product) SKU() string { return "" }

// Name returns the trimmed product name.
func (p Product) Name() string { return "" }

// Price returns the unit price.
func (p Product) Price() Money { return Money{} }

// Quantity returns the units in stock.
func (p Product) Quantity() int { return 0 }

// InStock reports whether at least one unit is in stock.
func (p Product) InStock() bool { return false }

// Restock returns a copy of p with count more units. count must be >= 1
// (ErrInvalidQuantity). The receiver is not modified.
func (p Product) Restock(count int) (Product, error) { return Product{}, errNotImplemented }

// Cart is a shopping cart in a single currency. Lines keep insertion order.
//
// The zero value is an empty cart whose currency is fixed by the first product
// added. NewCart fixes the currency up front.
type Cart struct {
	currency string
	lines    []cartLine
}

type cartLine struct {
	product  Product
	quantity int
}

// NewCart returns an empty cart in currency (ErrInvalidCurrency if malformed).
func NewCart(currency string) (*Cart, error) { return nil, errNotImplemented }

// Currency returns the cart currency, or "" for a zero-value cart with no lines.
func (c *Cart) Currency() string { return "" }

// Add puts quantity units of p in the cart. quantity must be >= 1
// (ErrInvalidQuantity), p must be a valid product (ErrInvalidSKU) and its price
// must be in the cart currency (ErrCurrencyMismatch). Adding a SKU already in the
// cart increases its quantity; the first Product snapshot is kept.
func (c *Cart) Add(p Product, quantity int) error { return errNotImplemented }

// Remove takes quantity units of sku out of the cart. Unknown SKUs are
// ErrNotInCart, quantities below 1 are ErrInvalidQuantity and removing more than
// held is ErrInsufficientQuantity. A line whose quantity reaches zero is deleted.
func (c *Cart) Remove(sku string, quantity int) error { return errNotImplemented }

// Quantity returns the units of sku in the cart, or 0 if absent.
func (c *Cart) Quantity(sku string) int { return 0 }

// Has reports whether sku is in the cart.
func (c *Cart) Has(sku string) bool { return false }

// Len returns the total number of units across all lines.
func (c *Cart) Len() int { return 0 }

// All returns an iterator over (sku, quantity) pairs in insertion order.
func (c *Cart) All() iter.Seq2[string, int] {
	return func(yield func(string, int) bool) {}
}

// Total returns the sum of price * quantity over all lines, in the cart currency.
// An empty cart totals to a zero Money in the cart currency. Overflow is ErrOverflow.
func (c *Cart) Total() (Money, error) { return Money{}, errNotImplemented }

// String returns a summary such as `Cart(currency="USD", items=3)`.
// It has a pointer receiver.
func (c *Cart) String() string { return "" }
