// Package structs is a Go refresher on structs, methods and interfaces: value
// types with checked constructors, composition by embedding, and the fmt and
// encoding interfaces that make a type print and serialise well.
package structs

import (
	"errors"
	"fmt"
)

// errNotImplemented is returned by the stubs. Replace every body.
var errNotImplemented = errors.New("not implemented")

var (
	// ErrInvalidCurrency reports a currency code that is not exactly three ASCII
	// uppercase letters, or a Money whose currency is missing where one is needed.
	ErrInvalidCurrency = errors.New("invalid currency")
	// ErrCurrencyMismatch reports an operation between amounts in different currencies.
	ErrCurrencyMismatch = errors.New("currency mismatch")
	// ErrInvalidAmount reports a malformed amount, or a negative one where that is not allowed.
	ErrInvalidAmount = errors.New("invalid amount")
	// ErrPrecision reports a decimal amount with non-zero digits beyond the currency's minor units.
	ErrPrecision = errors.New("amount exceeds currency precision")
	// ErrOverflow reports an arithmetic result outside the int64 range.
	ErrOverflow = errors.New("money overflow")
	// ErrInvalidArgument reports a non-money argument that is out of range, such as a zero divisor.
	ErrInvalidArgument = errors.New("invalid argument")
)

// Money is an exact amount of one currency, stored as an integer number of
// minor units (cents for USD, yen for JPY, fils for KWD).
//
// The zero value Money{} has no currency. It is not a valid amount for any
// currency: it does not mix with real currencies and it cannot be marshalled.
type Money struct {
	minor    int64
	currency string
}

// NewMoney returns an amount of minor units in currency. The currency must be
// three ASCII uppercase letters, otherwise ErrInvalidCurrency is returned.
func NewMoney(minor int64, currency string) (Money, error) {
	return Money{}, errNotImplemented
}

// ParseMoney parses a plain decimal amount such as "10.50", "-0.05" or "100".
// The optional sign is + or -. Digits beyond the currency's minor units must be
// zero ("1.500" USD is fine, "1.005" USD is ErrPrecision). Malformed input is
// ErrInvalidAmount and values outside the int64 minor-unit range are ErrOverflow.
func ParseMoney(amount, currency string) (Money, error) {
	return Money{}, errNotImplemented
}

// Minor returns the amount in minor units.
func (m Money) Minor() int64 { return 0 }

// Currency returns the three-letter currency code, or "" for the zero value.
func (m Money) Currency() string { return "" }

// IsZero reports whether the amount is zero. The currency is not considered.
func (m Money) IsZero() bool { return false }

// Add returns m + o. Currencies must match (ErrCurrencyMismatch) and the result
// must fit in int64 (ErrOverflow).
func (m Money) Add(o Money) (Money, error) { return Money{}, errNotImplemented }

// Sub returns m - o with the same checks as Add.
func (m Money) Sub(o Money) (Money, error) { return Money{}, errNotImplemented }

// Compare returns -1, 0 or +1 as m is less than, equal to or greater than o.
// Currencies must match (ErrCurrencyMismatch).
func (m Money) Compare(o Money) (int, error) { return 0, errNotImplemented }

// Scale returns m * num / den, rounded to the nearest minor unit with ties going
// to the even neighbour (banker's rounding). den must be non-zero
// (ErrInvalidArgument). The result must fit in int64 (ErrOverflow).
func (m Money) Scale(num, den int64) (Money, error) { return Money{}, errNotImplemented }

// Allocate splits m into n shares that sum exactly to m. Shares differ by at
// most one minor unit; the first shares receive the extra units. The sign of m
// applies to every share. n < 1 is ErrInvalidArgument.
func (m Money) Allocate(n int) ([]Money, error) { return nil, errNotImplemented }

// String returns the amount in plain decimal notation followed by the currency,
// for example "-0.05 USD". The zero value prints as "0.00".
func (m Money) String() string { return "" }

// Format implements fmt.Formatter. Verbs: %v and %s print String, %d prints the
// minor units, %#v prints GoString. Width and the '-' flag pad the text.
// Any other verb prints a "%!verb(...)" diagnostic.
func (m Money) Format(f fmt.State, verb rune) {}

// GoString implements fmt.GoStringer and returns a Go expression for m, such as
// `structs.NewMoney(1050, "USD")`.
func (m Money) GoString() string { return "" }

// MarshalText implements encoding.TextMarshaler. It emits the String form
// ("10.50 USD"). The zero value has no currency and returns ErrInvalidCurrency.
func (m Money) MarshalText() ([]byte, error) { return nil, errNotImplemented }

// UnmarshalText implements encoding.TextUnmarshaler. It parses the "<amount>
// <currency>" form. On error the receiver is left unchanged.
func (m *Money) UnmarshalText(text []byte) error { return errNotImplemented }
