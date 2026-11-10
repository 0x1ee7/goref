// Package typed holds the typed building blocks of a small service: domain
// IDs, enums, generic helpers, collections, a repository, an event bus and
// phantom-typed units. The spec is in README.md.
package typed

import "errors"

// ErrInvalidID reports a malformed, wrongly prefixed or non-positive identifier.
var ErrInvalidID = errors.New("invalid id")

// UserID identifies a user. Valid IDs are positive; zero means "unset".
type UserID int64

// OrderID identifies an order. Valid IDs are positive; zero means "unset".
type OrderID int64

// ParseUserID parses the text form "usr_<n>" (for example "usr_42").
// Malformed input, a different prefix or a non-positive number yield an error
// wrapping ErrInvalidID.
func ParseUserID(s string) (UserID, error) { return 0, nil }

// ParseOrderID parses the text form "ord_<n>" (for example "ord_7").
// Malformed input, a different prefix or a non-positive number yield an error
// wrapping ErrInvalidID.
func ParseOrderID(s string) (OrderID, error) { return 0, nil }

// String returns "usr_<n>" for any value, including zero and negative values.
func (id UserID) String() string { return "" }

// MarshalText returns the String form. Non-positive IDs yield an error
// wrapping ErrInvalidID.
func (id UserID) MarshalText() ([]byte, error) { return nil, nil }

// UnmarshalText parses the "usr_<n>" form into id. On error id is unchanged.
func (id *UserID) UnmarshalText(text []byte) error { return nil }

// String returns "ord_<n>" for any value, including zero and negative values.
func (id OrderID) String() string { return "" }

// MarshalText returns the String form. Non-positive IDs yield an error
// wrapping ErrInvalidID.
func (id OrderID) MarshalText() ([]byte, error) { return nil, nil }

// UnmarshalText parses the "ord_<n>" form into id. On error id is unchanged.
func (id *OrderID) UnmarshalText(text []byte) error { return nil }
