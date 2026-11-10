package typed

import (
	"errors"
	"iter"
)

// ErrInvalidOrderStatus reports an unknown, unspecified or malformed status.
var ErrInvalidOrderStatus = errors.New("invalid order status")

// OrderStatus is the lifecycle state of an order. The zero value is
// OrderStatusUnspecified, which is never a valid state: an unset field must
// not silently serialize as a real status.
type OrderStatus int

// Order lifecycle states, in declaration order.
const (
	OrderStatusUnspecified OrderStatus = iota
	OrderStatusPending
	OrderStatusPaid
	OrderStatusShipped
	OrderStatusCancelled
)

// ParseOrderStatus parses a status name. Surrounding whitespace is trimmed and
// matching is case-insensitive. "unspecified" and unknown names yield an error
// wrapping ErrInvalidOrderStatus.
func ParseOrderStatus(s string) (OrderStatus, error) { return OrderStatusUnspecified, nil }

// String returns the lowercase name ("pending", ...). Unspecified renders as
// "unspecified"; unknown values render as "OrderStatus(<n>)".
func (s OrderStatus) String() string { return "" }

// Valid reports whether s is one of the real lifecycle states.
func (s OrderStatus) Valid() bool { return false }

// MarshalText returns the name of a valid status. Invalid values yield an
// error wrapping ErrInvalidOrderStatus.
func (s OrderStatus) MarshalText() ([]byte, error) { return nil, nil }

// UnmarshalText parses a status name using ParseOrderStatus rules. On error s
// is unchanged.
func (s *OrderStatus) UnmarshalText(text []byte) error { return nil }

// CanTransition reports whether an order may move from s to next.
// Invalid states on either side never transition.
func (s OrderStatus) CanTransition(next OrderStatus) bool { return false }

// AllOrderStatuses yields the valid states in declaration order.
func AllOrderStatuses() iter.Seq[OrderStatus] {
	return func(yield func(OrderStatus) bool) {}
}
