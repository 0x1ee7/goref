package functional

import "errors"

// Sentinel errors returned by CartReducer. Use errors.Is.
var (
	ErrUnknownAction   = errors.New("unknown cart action")
	ErrInvalidQuantity = errors.New("quantity out of range")
	ErrInvalidPrice    = errors.New("negative unit price")
	ErrUnknownSKU      = errors.New("unknown sku")
	ErrUnknownCoupon   = errors.New("unknown coupon")
)

// LineItem is one SKU in a cart. Labels is a reference type: copying a LineItem
// copies the map header, not the entries.
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

// NewAddItem builds an AddItem with quantity 1.
func NewAddItem(sku, name string, unitPriceCents int64) AddItem {
	return AddItem{}
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

// CouponPercent reports the percent off for code.
func CouponPercent(code string) (int, bool) {
	return 0, false
}

// CartReducer returns the next state for action. It never modifies state or
// action, and the returned Cart shares no memory with either. On error the
// returned Cart is the zero value and must be ignored.
func CartReducer(state Cart, action any) (Cart, error) {
	return Cart{}, nil
}

// CartSubtotal is the sum of unit price times quantity over all lines.
func CartSubtotal(cart Cart) int64 {
	return 0
}

// CartTotal is the subtotal minus the coupon discount. The discount is floored
// to whole cents. An unknown coupon in state grants no discount.
func CartTotal(cart Cart) int64 {
	return 0
}

// RunActions folds actions over state with CartReducer and stops at the first
// error. The actions slice is not modified.
func RunActions(state Cart, actions []any) (Cart, error) {
	return Cart{}, nil
}
