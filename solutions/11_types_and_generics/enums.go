package typed

import (
	"errors"
	"fmt"
	"iter"
	"strings"
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

var orderStatusNames = map[OrderStatus]string{
	OrderStatusUnspecified: "unspecified",
	OrderStatusPending:     "pending",
	OrderStatusPaid:        "paid",
	OrderStatusShipped:     "shipped",
	OrderStatusCancelled:   "cancelled",
}

// ParseOrderStatus parses a status name. Surrounding whitespace is trimmed and
// matching is case-insensitive. "unspecified" and unknown names yield an error
// wrapping ErrInvalidOrderStatus.
func ParseOrderStatus(s string) (OrderStatus, error) {
	name := strings.ToLower(strings.TrimSpace(s))
	for st := range AllOrderStatuses() {
		if orderStatusNames[st] == name {
			return st, nil
		}
	}
	return OrderStatusUnspecified, fmt.Errorf("parse order status %q: %w", s, ErrInvalidOrderStatus)
}

// String returns the lowercase name ("pending", ...). Unspecified renders as
// "unspecified"; unknown values render as "OrderStatus(<n>)".
func (s OrderStatus) String() string {
	if name, ok := orderStatusNames[s]; ok {
		return name
	}
	return fmt.Sprintf("OrderStatus(%d)", int(s))
}

// Valid reports whether s is one of the real lifecycle states.
func (s OrderStatus) Valid() bool {
	return s >= OrderStatusPending && s <= OrderStatusCancelled
}

// MarshalText returns the name of a valid status. Invalid values yield an
// error wrapping ErrInvalidOrderStatus.
func (s OrderStatus) MarshalText() ([]byte, error) {
	if !s.Valid() {
		return nil, fmt.Errorf("marshal order status %s: %w", s, ErrInvalidOrderStatus)
	}
	return []byte(orderStatusNames[s]), nil
}

// UnmarshalText parses a status name using ParseOrderStatus rules. On error s
// is unchanged.
func (s *OrderStatus) UnmarshalText(text []byte) error {
	v, err := ParseOrderStatus(string(text))
	if err != nil {
		return err
	}
	*s = v
	return nil
}

// CanTransition reports whether an order may move from s to next.
// Invalid states on either side never transition.
func (s OrderStatus) CanTransition(next OrderStatus) bool {
	switch s {
	case OrderStatusPending:
		return next == OrderStatusPaid || next == OrderStatusCancelled
	case OrderStatusPaid:
		return next == OrderStatusShipped || next == OrderStatusCancelled
	}
	return false
}

// AllOrderStatuses yields the valid states in declaration order.
func AllOrderStatuses() iter.Seq[OrderStatus] {
	return func(yield func(OrderStatus) bool) {
		for s := OrderStatusPending; s <= OrderStatusCancelled; s++ {
			if !yield(s) {
				return
			}
		}
	}
}
