// Package structs models a small domain (money, a shopping cart, bank accounts,
// employee records and semantic versions) to show how Go structs, methods and
// constructors enforce invariants.
package structs

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"strconv"
	"strings"
)

var (
	// ErrInvalidCurrency reports a currency code that is not three upper-case ASCII letters.
	ErrInvalidCurrency = errors.New("invalid currency code")
	// ErrCurrencyMismatch reports an operation between two different currencies.
	ErrCurrencyMismatch = errors.New("currency mismatch")
	// ErrInvalidAmount reports text that is not a decimal amount.
	ErrInvalidAmount = errors.New("invalid amount")
	// ErrPrecision reports an amount with more fractional digits than its currency has minor units.
	ErrPrecision = errors.New("amount has more precision than the currency allows")
	// ErrOverflow reports arithmetic that does not fit in int64 minor units.
	ErrOverflow = errors.New("money overflow")
	// ErrInvalidArgument reports a zero denominator, a non-positive share count or a similar bad argument.
	ErrInvalidArgument = errors.New("invalid argument")
)

// minorExponents lists currencies whose minor-unit exponent is not the usual 2.
var minorExponents = map[string]int{
	"CLP": 0, "ISK": 0, "JPY": 0, "KRW": 0, "VND": 0,
	"BHD": 3, "IQD": 3, "JOD": 3, "KWD": 3, "OMR": 3, "TND": 3,
}

// Money is an amount of one currency stored in integer minor units (cents,
// yen, fils). Float64 cannot represent 0.10 exactly, so money is never a float.
//
// The zero value is Money{}: no currency, zero amount. It does not mix with real
// currencies. Use NewMoney or ParseMoney to build a usable value.
type Money struct {
	minor    int64
	currency string
}

// validCurrency reports whether c is three upper-case ASCII letters.
func validCurrency(c string) bool {
	if len(c) != 3 {
		return false
	}
	for i := range len(c) {
		if c[i] < 'A' || c[i] > 'Z' {
			return false
		}
	}
	return true
}

// minorDigits returns the number of fractional digits for currency. Currencies
// not in minorExponents use 2.
func minorDigits(currency string) int {
	if d, ok := minorExponents[currency]; ok {
		return d
	}
	return 2
}

// NewMoney builds a Money from minor units. The currency must be valid.
func NewMoney(minor int64, currency string) (Money, error) {
	if !validCurrency(currency) {
		return Money{}, fmt.Errorf("%w: %q", ErrInvalidCurrency, currency)
	}
	return Money{minor: minor, currency: currency}, nil
}

// ParseMoney parses a decimal string such as "10.50" in the given currency.
// It accepts an optional sign, at least one integer digit, and an optional
// fraction. Extra fractional digits are allowed only if they are all zero.
func ParseMoney(amount, currency string) (Money, error) {
	if !validCurrency(currency) {
		return Money{}, fmt.Errorf("%w: %q", ErrInvalidCurrency, currency)
	}
	digits := minorDigits(currency)

	s, neg := amount, false
	switch {
	case strings.HasPrefix(s, "-"):
		neg, s = true, s[1:]
	case strings.HasPrefix(s, "+"):
		s = s[1:]
	}
	intPart, fracPart, hasDot := strings.Cut(s, ".")
	if !allDigits(intPart) {
		return Money{}, fmt.Errorf("%w: %q", ErrInvalidAmount, amount)
	}
	if hasDot && !allDigits(fracPart) {
		return Money{}, fmt.Errorf("%w: %q", ErrInvalidAmount, amount)
	}
	if len(fracPart) > digits {
		if strings.Trim(fracPart[digits:], "0") != "" {
			return Money{}, fmt.Errorf("%w: %q has more than %d fractional digits", ErrPrecision, amount, digits)
		}
		fracPart = fracPart[:digits]
	}
	fracPart += strings.Repeat("0", digits-len(fracPart))

	mag, err := strconv.ParseUint(intPart+fracPart, 10, 64)
	if errors.Is(err, strconv.ErrRange) {
		return Money{}, fmt.Errorf("%w: %q", ErrOverflow, amount)
	}
	if err != nil {
		return Money{}, fmt.Errorf("%w: %q", ErrInvalidAmount, amount)
	}
	minor, ok := signedFromMagnitude(mag, neg)
	if !ok {
		return Money{}, fmt.Errorf("%w: %q", ErrOverflow, amount)
	}
	return Money{minor: minor, currency: currency}, nil
}

// allDigits reports whether s is non-empty and contains only ASCII digits.
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// signedFromMagnitude converts |value| and its sign to int64. It reports false
// when the value does not fit. The negative case handles math.MinInt64, whose
// magnitude does not fit in int64.
func signedFromMagnitude(mag uint64, neg bool) (int64, bool) {
	if !neg {
		if mag > math.MaxInt64 {
			return 0, false
		}
		return int64(mag), true
	}
	if mag == 0 {
		return 0, true
	}
	if mag > 1<<63 {
		return 0, false
	}
	return -int64(mag-1) - 1, true
}

// Minor returns the amount in minor units.
func (m Money) Minor() int64 { return m.minor }

// Currency returns the three-letter currency code, or "" for the zero value.
func (m Money) Currency() string { return m.currency }

// IsZero reports whether the amount is zero, regardless of currency.
func (m Money) IsZero() bool { return m.minor == 0 }

// Add returns m + o. Both values must share a currency. The result is checked
// for int64 overflow.
func (m Money) Add(o Money) (Money, error) {
	if m.currency != o.currency {
		return Money{}, fmt.Errorf("%w: %q and %q", ErrCurrencyMismatch, m.currency, o.currency)
	}
	sum := m.minor + o.minor
	if (o.minor > 0 && sum < m.minor) || (o.minor < 0 && sum > m.minor) {
		return Money{}, ErrOverflow
	}
	return Money{minor: sum, currency: m.currency}, nil
}

// Sub returns m - o. Both values must share a currency. The result is checked
// for int64 overflow.
func (m Money) Sub(o Money) (Money, error) {
	if m.currency != o.currency {
		return Money{}, fmt.Errorf("%w: %q and %q", ErrCurrencyMismatch, m.currency, o.currency)
	}
	diff := m.minor - o.minor
	if (o.minor > 0 && diff > m.minor) || (o.minor < 0 && diff < m.minor) {
		return Money{}, ErrOverflow
	}
	return Money{minor: diff, currency: m.currency}, nil
}

// Compare returns -1, 0 or +1 as m is less than, equal to or greater than o.
func (m Money) Compare(o Money) (int, error) {
	if m.currency != o.currency {
		return 0, fmt.Errorf("%w: %q and %q", ErrCurrencyMismatch, m.currency, o.currency)
	}
	return cmp.Compare(m.minor, o.minor), nil
}

// Scale returns m * num / den, rounded to the nearest minor unit with ties to
// even (banker's rounding). This is the rounding that avoids a drift of +0.5
// units over many operations. den must not be zero.
func (m Money) Scale(num, den int64) (Money, error) {
	if den == 0 {
		return Money{}, fmt.Errorf("%w: zero denominator", ErrInvalidArgument)
	}
	n := new(big.Int).Mul(big.NewInt(m.minor), big.NewInt(num))
	r := new(big.Rat).SetFrac(n, big.NewInt(den))
	q := roundHalfEven(r)
	if !q.IsInt64() {
		return Money{}, ErrOverflow
	}
	return Money{minor: q.Int64(), currency: m.currency}, nil
}

// roundHalfEven rounds r to the nearest integer, with ties going to the even
// neighbour.
func roundHalfEven(r *big.Rat) *big.Int {
	q, rem := new(big.Int).QuoRem(r.Num(), r.Denom(), new(big.Int))
	twiceRem := new(big.Int).Abs(rem)
	twiceRem.Lsh(twiceRem, 1)
	switch c := twiceRem.Cmp(r.Denom()); {
	case c > 0, c == 0 && q.Bit(0) == 1:
		if r.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	return q
}

// Allocate splits m into n shares that sum exactly to m. Each share differs by
// at most one minor unit, and the first remainder units go to the first shares.
// This avoids losing cents when splitting a bill. n must be at least 1.
func (m Money) Allocate(n int) ([]Money, error) {
	if n < 1 {
		return nil, fmt.Errorf("%w: share count %d", ErrInvalidArgument, n)
	}
	mag := uint64(m.minor)
	if m.minor < 0 {
		mag = uint64(-(m.minor + 1)) + 1
	}
	count := uint64(n)
	base, rem := mag/count, mag%count

	shares := make([]Money, n)
	for i := range shares {
		s := base
		if uint64(i) < rem {
			s++
		}
		v, _ := signedFromMagnitude(s, m.minor < 0)
		shares[i] = Money{minor: v, currency: m.currency}
	}
	return shares, nil
}

// String returns the amount with its currency, such as "10.50 USD". The zero
// value prints as "0.00".
func (m Money) String() string {
	amount := formatAmount(m.minor, minorDigits(m.currency))
	if m.currency == "" {
		return amount
	}
	return amount + " " + m.currency
}

// formatAmount renders minor units with the given number of fractional digits.
func formatAmount(minor int64, digits int) string {
	sign := ""
	mag := uint64(minor)
	if minor < 0 {
		sign = "-"
		mag = uint64(-(minor + 1)) + 1
	}
	s := strconv.FormatUint(mag, 10)
	if digits == 0 {
		return sign + s
	}
	for len(s) <= digits {
		s = "0" + s
	}
	return sign + s[:len(s)-digits] + "." + s[len(s)-digits:]
}

// Format implements fmt.Formatter. It handles %v and %s (with width and the
// '-' flag), %d (raw minor units) and %#v (GoString). Other verbs print a
// diagnostic. Formatter takes precedence over GoStringer and Stringer.
func (m Money) Format(f fmt.State, verb rune) {
	switch verb {
	case 'v':
		if f.Flag('#') {
			pad(f, m.GoString())
			return
		}
		pad(f, m.String())
	case 's':
		pad(f, m.String())
	case 'd':
		pad(f, strconv.FormatInt(m.minor, 10))
	default:
		fmt.Fprintf(f, "%%!%c(structs.Money=%s)", verb, m.String())
	}
}

// pad writes s to f, padding it to the requested width. Right-aligned by
// default, left-aligned with the '-' flag.
func pad(f fmt.State, s string) {
	w, ok := f.Width()
	if !ok || len(s) >= w {
		io.WriteString(f, s)
		return
	}
	fill := strings.Repeat(" ", w-len(s))
	if f.Flag('-') {
		io.WriteString(f, s+fill)
		return
	}
	io.WriteString(f, fill+s)
}

// GoString returns a Go expression that rebuilds the value, for %#v.
func (m Money) GoString() string {
	if m.currency == "" && m.minor == 0 {
		return "structs.Money{}"
	}
	return fmt.Sprintf("structs.NewMoney(%d, %q)", m.minor, m.currency)
}

// MarshalText implements encoding.TextMarshaler using the String form. The
// zero value has no currency, so it cannot be marshalled.
func (m Money) MarshalText() ([]byte, error) {
	if !validCurrency(m.currency) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidCurrency, m.currency)
	}
	return []byte(m.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler. It expects "AMOUNT CUR",
// such as "10.50 USD". On error the receiver is unchanged.
func (m *Money) UnmarshalText(text []byte) error {
	amount, currency, ok := strings.Cut(string(text), " ")
	if !ok {
		return fmt.Errorf("%w: want \"AMOUNT CUR\", got %q", ErrInvalidAmount, text)
	}
	parsed, err := ParseMoney(amount, currency)
	if err != nil {
		return err
	}
	*m = parsed
	return nil
}
