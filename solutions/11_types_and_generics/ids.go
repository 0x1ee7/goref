// Package typed holds the typed building blocks of a small service: domain
// IDs, enums, generic helpers, collections, a repository, an event bus and
// phantom-typed units. The spec is in README.md.
package typed

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidID reports a malformed, wrongly prefixed or non-positive identifier.
var ErrInvalidID = errors.New("invalid id")

// UserID identifies a user. Valid IDs are positive; zero means "unset".
type UserID int64

// OrderID identifies an order. Valid IDs are positive; zero means "unset".
type OrderID int64

// ParseUserID parses the text form "usr_<n>" (for example "usr_42").
// Malformed input, a different prefix or a non-positive number yield an error
// wrapping ErrInvalidID.
func ParseUserID(s string) (UserID, error) {
	n, err := parseID(s, "usr_")
	if err != nil {
		return 0, err
	}
	return UserID(n), nil
}

// ParseOrderID parses the text form "ord_<n>" (for example "ord_7").
// Malformed input, a different prefix or a non-positive number yield an error
// wrapping ErrInvalidID.
func ParseOrderID(s string) (OrderID, error) {
	n, err := parseID(s, "ord_")
	if err != nil {
		return 0, err
	}
	return OrderID(n), nil
}

// String returns "usr_<n>" for any value, including zero and negative values.
func (id UserID) String() string { return formatID("usr_", int64(id)) }

// MarshalText returns the String form. Non-positive IDs yield an error
// wrapping ErrInvalidID.
func (id UserID) MarshalText() ([]byte, error) { return marshalID("usr_", int64(id)) }

// UnmarshalText parses the "usr_<n>" form into id. On error id is unchanged.
func (id *UserID) UnmarshalText(text []byte) error {
	v, err := ParseUserID(string(text))
	if err != nil {
		return err
	}
	*id = v
	return nil
}

// String returns "ord_<n>" for any value, including zero and negative values.
func (id OrderID) String() string { return formatID("ord_", int64(id)) }

// MarshalText returns the String form. Non-positive IDs yield an error
// wrapping ErrInvalidID.
func (id OrderID) MarshalText() ([]byte, error) { return marshalID("ord_", int64(id)) }

// UnmarshalText parses the "ord_<n>" form into id. On error id is unchanged.
func (id *OrderID) UnmarshalText(text []byte) error {
	v, err := ParseOrderID(string(text))
	if err != nil {
		return err
	}
	*id = v
	return nil
}

func parseID(s, prefix string) (int64, error) {
	digits, ok := strings.CutPrefix(s, prefix)
	if !ok {
		return 0, fmt.Errorf("parse id %q: want prefix %q: %w", s, prefix, ErrInvalidID)
	}
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse id %q: %w: %w", s, ErrInvalidID, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("parse id %q: must be positive: %w", s, ErrInvalidID)
	}
	return n, nil
}

func formatID(prefix string, n int64) string {
	return prefix + strconv.FormatInt(n, 10)
}

func marshalID(prefix string, n int64) ([]byte, error) {
	if n <= 0 {
		return nil, fmt.Errorf("marshal id %s: %w", formatID(prefix, n), ErrInvalidID)
	}
	return []byte(formatID(prefix, n)), nil
}
