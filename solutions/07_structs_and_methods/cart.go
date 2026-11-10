package structs

import (
	"errors"
	"fmt"
	"iter"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

var (
	// ErrInvalidSKU reports a SKU outside the allowed pattern, or an empty one.
	ErrInvalidSKU = errors.New("invalid SKU")
	// ErrInvalidName reports a blank or over-long product name.
	ErrInvalidName = errors.New("invalid product name")
	// ErrInvalidQuantity reports a quantity that is negative, zero where a positive
	// value is needed, or would overflow.
	ErrInvalidQuantity = errors.New("invalid quantity")
	// ErrNotInCart reports a SKU that the cart does not hold.
	ErrNotInCart = errors.New("product not in cart")
	// ErrInsufficientQuantity reports a removal larger than the quantity held.
	ErrInsufficientQuantity = errors.New("insufficient quantity in cart")
)

// skuPattern allows 4 to 16 characters: upper-case letters, digits and hyphens,
// starting with a letter or digit.
var skuPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]{3,15}$`)

// maxNameRunes is the longest product name accepted, counted in runes.
const maxNameRunes = 100

// Product is an immutable catalogue entry: SKU, name, unit price and stock
// quantity. Copying a Product is cheap and safe. All fields are unexported, so
// a Product can only be built by NewProduct and Restock.
type Product struct {
	sku      string
	name     string
	price    Money
	quantity int
}

// NewProduct validates its inputs and returns a Product. The name is trimmed.
// The price must have a valid currency and must not be negative.
func NewProduct(sku, name string, price Money, quantity int) (Product, error) {
	if !skuPattern.MatchString(sku) {
		return Product{}, fmt.Errorf("%w: %q", ErrInvalidSKU, sku)
	}
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n == 0 || n > maxNameRunes {
		return Product{}, fmt.Errorf("%w: %d runes", ErrInvalidName, n)
	}
	if !validCurrency(price.currency) {
		return Product{}, fmt.Errorf("%w: %q", ErrInvalidCurrency, price.currency)
	}
	if price.minor < 0 {
		return Product{}, fmt.Errorf("%w: negative price", ErrInvalidArgument)
	}
	if quantity < 0 {
		return Product{}, fmt.Errorf("%w: %d", ErrInvalidQuantity, quantity)
	}
	return Product{sku: sku, name: name, price: price, quantity: quantity}, nil
}

// SKU returns the stock-keeping unit.
func (p Product) SKU() string { return p.sku }

// Name returns the trimmed product name.
func (p Product) Name() string { return p.name }

// Price returns the unit price.
func (p Product) Price() Money { return p.price }

// Quantity returns the stock on hand.
func (p Product) Quantity() int { return p.quantity }

// InStock reports whether any units are on hand.
func (p Product) InStock() bool { return p.quantity > 0 }

// Restock returns a copy of p with count more units. The receiver is unchanged.
func (p Product) Restock(count int) (Product, error) {
	if count < 1 {
		return Product{}, fmt.Errorf("%w: restock count %d", ErrInvalidQuantity, count)
	}
	if p.quantity > math.MaxInt-count {
		return Product{}, fmt.Errorf("%w: stock would overflow", ErrInvalidQuantity)
	}
	p.quantity += count
	return p, nil
}

// cartLine is one SKU in a cart. It keeps the Product snapshot from the first Add.
type cartLine struct {
	product  Product
	quantity int
}

// Cart is a shopping basket in one currency. A Cart built with NewCart has a
// fixed currency. The zero value is an empty cart that takes its currency from
// the first product added.
//
// Cart holds no lock. It is not safe for concurrent use.
type Cart struct {
	currency string
	lines    []cartLine
}

// NewCart returns an empty cart that accepts products in currency.
func NewCart(currency string) (*Cart, error) {
	if !validCurrency(currency) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidCurrency, currency)
	}
	return &Cart{currency: currency}, nil
}

// Currency returns the cart's currency, or "" for a zero-value cart with no items.
func (c *Cart) Currency() string { return c.currency }

// Add puts qty units of p in the cart. Re-adding a SKU increases its quantity and
// keeps the Product from the first Add. A product in another currency gives
// ErrCurrencyMismatch and leaves the cart unchanged.
func (c *Cart) Add(p Product, qty int) error {
	if p.sku == "" {
		return ErrInvalidSKU
	}
	if qty < 1 {
		return fmt.Errorf("%w: add quantity %d", ErrInvalidQuantity, qty)
	}
	cur := c.currency
	if cur == "" {
		cur = p.price.currency
	}
	if p.price.currency != cur {
		return fmt.Errorf("%w: cart is %q, product is %q", ErrCurrencyMismatch, cur, p.price.currency)
	}
	i := c.index(p.sku)
	if i >= 0 {
		if qty > math.MaxInt-c.lines[i].quantity {
			return fmt.Errorf("%w: quantity would overflow", ErrInvalidQuantity)
		}
		c.lines[i].quantity += qty
	} else {
		c.lines = append(c.lines, cartLine{product: p, quantity: qty})
	}
	c.currency = cur
	return nil
}

// Remove takes qty units of sku out of the cart. A line whose quantity reaches
// zero is deleted. Removing more than the cart holds gives ErrInsufficientQuantity
// and leaves the cart unchanged.
func (c *Cart) Remove(sku string, qty int) error {
	if qty < 1 {
		return fmt.Errorf("%w: remove quantity %d", ErrInvalidQuantity, qty)
	}
	i := c.index(sku)
	if i < 0 {
		return fmt.Errorf("%w: %q", ErrNotInCart, sku)
	}
	if qty > c.lines[i].quantity {
		return fmt.Errorf("%w: have %d of %q, removing %d", ErrInsufficientQuantity, c.lines[i].quantity, sku, qty)
	}
	c.lines[i].quantity -= qty
	if c.lines[i].quantity == 0 {
		c.lines = slices.Delete(c.lines, i, i+1)
	}
	return nil
}

// Quantity returns the units of sku in the cart, or 0 if absent.
func (c *Cart) Quantity(sku string) int {
	if i := c.index(sku); i >= 0 {
		return c.lines[i].quantity
	}
	return 0
}

// Has reports whether the cart holds at least one unit of sku.
func (c *Cart) Has(sku string) bool { return c.index(sku) >= 0 }

// Len returns the total number of units across all lines.
func (c *Cart) Len() int {
	n := 0
	for _, l := range c.lines {
		n += l.quantity
	}
	return n
}

// All yields each SKU and its quantity in insertion order. The iterator reads
// the cart lazily, so do not modify the cart during iteration.
func (c *Cart) All() iter.Seq2[string, int] {
	return func(yield func(string, int) bool) {
		for _, l := range c.lines {
			if !yield(l.product.sku, l.quantity) {
				return
			}
		}
	}
}

// Total sums price times quantity over all lines, using checked arithmetic.
// An empty cart returns a zero Money in the cart's currency.
func (c *Cart) Total() (Money, error) {
	total := Money{currency: c.currency}
	for _, l := range c.lines {
		line, err := l.product.price.Scale(int64(l.quantity), 1)
		if err != nil {
			return Money{}, err
		}
		if total, err = total.Add(line); err != nil {
			return Money{}, err
		}
	}
	return total, nil
}

// String describes the cart without listing its lines, for example
// Cart(currency="USD", items=3).
func (c *Cart) String() string {
	return fmt.Sprintf("Cart(currency=%q, items=%d)", c.currency, c.Len())
}

// index returns the position of sku in c.lines, or -1.
func (c *Cart) index(sku string) int {
	return slices.IndexFunc(c.lines, func(l cartLine) bool { return l.product.sku == sku })
}
