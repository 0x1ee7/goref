package functional

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

// Sentinel errors returned by CartReducer. Use errors.Is.
var (
	ErrUnknownAction   = errors.New("unknown cart action")
	ErrInvalidQuantity = errors.New("quantity out of range")
	ErrInvalidPrice    = errors.New("negative unit price")
	ErrUnknownSKU      = errors.New("unknown sku")
	ErrUnknownCoupon   = errors.New("unknown coupon")
)

// LineItem is one SKU in a cart. Labels is a reference type: copying a LineItem
// copies the map header, not the entries (see cloneItems).
type LineItem struct {
	SKU            string
	Name           string
	UnitPriceCents int64
	Quantity       int
	Labels         map[string]string
}

// Cart is the reducer state. The zero value is an empty cart with no coupon.
type Cart struct {
	Items  []LineItem
	Coupon string // empty means no coupon
}

// Actions are plain data. CartReducer switches on their concrete type.

// AddItem appends a line, or increases the quantity of an existing SKU.
type AddItem struct {
	SKU            string
	Name           string
	UnitPriceCents int64
	Quantity       int
	Labels         map[string]string
}

// NewAddItem builds an AddItem with quantity 1. Go has no default arguments; a
// constructor is the idiomatic way to supply one.
func NewAddItem(sku, name string, unitPriceCents int64) AddItem {
	return AddItem{SKU: sku, Name: name, UnitPriceCents: unitPriceCents, Quantity: 1}
}

// RemoveItem drops the line for SKU. Unknown SKUs are a no-op.
type RemoveItem struct{ SKU string }

// SetQuantity sets a line's quantity; zero removes the line.
type SetQuantity struct {
	SKU      string
	Quantity int
}

// ApplyCoupon sets the coupon; the empty code clears it.
type ApplyCoupon struct{ Code string }

// ClearCart resets the cart to its zero value.
type ClearCart struct{}

var coupons = map[string]int{"SAVE10": 10, "HALF": 50}

// CouponPercent reports the percent off for code. The table is unexported, so
// callers cannot mutate it; that is the Go equivalent of a read-only mapping.
func CouponPercent(code string) (int, bool) {
	pct, ok := coupons[code]
	return pct, ok
}

// CartReducer returns the next state for action. It never modifies state or
// action, and the returned Cart shares no memory with either. On error the
// returned Cart is the zero value and must be ignored.
func CartReducer(state Cart, action any) (Cart, error) {
	switch a := action.(type) {
	case AddItem:
		return addItem(state, a)
	case RemoveItem:
		items := cloneItems(state.Items)
		items = slices.DeleteFunc(items, func(li LineItem) bool { return li.SKU == a.SKU })
		return withItems(state.Coupon, items), nil
	case SetQuantity:
		if a.Quantity < 0 {
			return Cart{}, fmt.Errorf("%w: %d", ErrInvalidQuantity, a.Quantity)
		}
		i := indexOfSKU(state.Items, a.SKU)
		if i < 0 {
			return Cart{}, fmt.Errorf("%w: %q", ErrUnknownSKU, a.SKU)
		}
		items := cloneItems(state.Items)
		if a.Quantity == 0 {
			items = slices.Delete(items, i, i+1)
		} else {
			items[i].Quantity = a.Quantity
		}
		return withItems(state.Coupon, items), nil
	case ApplyCoupon:
		if a.Code != "" {
			if _, ok := CouponPercent(a.Code); !ok {
				return Cart{}, fmt.Errorf("%w: %q", ErrUnknownCoupon, a.Code)
			}
		}
		return withItems(a.Code, cloneItems(state.Items)), nil
	case ClearCart:
		return Cart{}, nil
	default:
		return Cart{}, fmt.Errorf("%w: %T", ErrUnknownAction, action)
	}
}

func addItem(state Cart, a AddItem) (Cart, error) {
	if a.Quantity < 1 {
		return Cart{}, fmt.Errorf("%w: %d", ErrInvalidQuantity, a.Quantity)
	}
	if a.UnitPriceCents < 0 {
		return Cart{}, fmt.Errorf("%w: %d", ErrInvalidPrice, a.UnitPriceCents)
	}
	items := cloneItems(state.Items)
	if i := indexOfSKU(items, a.SKU); i >= 0 {
		// The existing line keeps its position, name, price and labels.
		items[i].Quantity += a.Quantity
	} else {
		items = append(items, LineItem{
			SKU:            a.SKU,
			Name:           a.Name,
			UnitPriceCents: a.UnitPriceCents,
			Quantity:       a.Quantity,
			Labels:         maps.Clone(a.Labels), // do not alias the caller's map
		})
	}
	return withItems(state.Coupon, items), nil
}

// cloneItems deep-copies the slice and each line's Labels map. slices.Clone
// alone would be a shallow copy: the maps would still be shared.
func cloneItems(items []LineItem) []LineItem {
	out := make([]LineItem, len(items))
	for i, li := range items {
		li.Labels = maps.Clone(li.Labels)
		out[i] = li
	}
	return out
}

// withItems normalises an empty item list to nil so that an emptied cart equals
// a fresh one.
func withItems(coupon string, items []LineItem) Cart {
	if len(items) == 0 {
		items = nil
	}
	return Cart{Items: items, Coupon: coupon}
}

func indexOfSKU(items []LineItem, sku string) int {
	return slices.IndexFunc(items, func(li LineItem) bool { return li.SKU == sku })
}

// CartSubtotal is the sum of unit price times quantity over all lines.
func CartSubtotal(cart Cart) int64 {
	return Reduce(cart.Items, int64(0), func(acc int64, li LineItem) int64 {
		return acc + li.UnitPriceCents*int64(li.Quantity)
	})
}

// CartTotal is the subtotal minus the coupon discount. The discount is floored
// to whole cents. An unknown coupon in state grants no discount.
func CartTotal(cart Cart) int64 {
	subtotal := CartSubtotal(cart)
	pct, ok := CouponPercent(cart.Coupon)
	if !ok {
		return subtotal
	}
	return subtotal - subtotal*int64(pct)/100
}

// RunActions folds actions over state with CartReducer and stops at the first
// error. The actions slice is not modified.
func RunActions(state Cart, actions []any) (Cart, error) {
	return TryReduce(actions, state, CartReducer)
}
