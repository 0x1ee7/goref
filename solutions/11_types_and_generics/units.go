package typed

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ErrInvalidDistance reports text that is not a finite number followed by a unit symbol.
var ErrInvalidDistance = errors.New("invalid distance")

// ErrUnitMismatch reports a parsed unit symbol that differs from the target unit.
var ErrUnitMismatch = errors.New("unit mismatch")

// Unit is implemented by marker types that describe a unit of length.
type Unit interface {
	Symbol() string
	MetersPerUnit() float64
}

// Meters is the SI unit of length. Symbol "m".
type Meters struct{}

// Symbol returns "m".
func (Meters) Symbol() string { return "m" }

// MetersPerUnit returns 1.
func (Meters) MetersPerUnit() float64 { return 1 }

// Feet is the international foot. Symbol "ft", exactly 0.3048 m.
type Feet struct{}

// Symbol returns "ft".
func (Feet) Symbol() string { return "ft" }

// MetersPerUnit returns 0.3048.
func (Feet) MetersPerUnit() float64 { return 0.3048 }

// Distance is a length measured in unit U. U is a phantom parameter: it does
// not appear in the underlying type, so Distance[Meters] and Distance[Feet]
// are distinct types with the same float64 representation.
type Distance[U Unit] float64

// Value returns the numeric magnitude in U.
func (d Distance[U]) Value() float64 { return float64(d) }

// Add returns d+other. Only distances in the same unit can be added.
func (d Distance[U]) Add(other Distance[U]) Distance[U] { return d + other }

// String returns the magnitude and symbol separated by one space, for example
// "12.5 m". The magnitude uses the shortest exact float64 form.
func (d Distance[U]) String() string {
	var u U
	return strconv.FormatFloat(float64(d), 'g', -1, 64) + " " + u.Symbol()
}

// MarshalText returns the String form.
func (d Distance[U]) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText parses text with ParseDistance. On error d is unchanged.
func (d *Distance[U]) UnmarshalText(text []byte) error {
	v, err := ParseDistance[U](string(text))
	if err != nil {
		return err
	}
	*d = v
	return nil
}

// ParseDistance parses "<number>[ ]<symbol>" in unit U. Surrounding whitespace is
// ignored. The number must be finite. A symbol other than U.Symbol() yields
// ErrUnitMismatch; any other malformation yields ErrInvalidDistance.
func ParseDistance[U Unit](s string) (Distance[U], error) {
	s = strings.TrimSpace(s)
	// The symbol is the trailing run of letters; everything before it is the number.
	i := len(s)
	for i > 0 && isASCIILetter(s[i-1]) {
		i--
	}
	numPart := strings.TrimSpace(s[:i])
	symbol := s[i:]

	f, err := strconv.ParseFloat(numPart, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("parse distance %q: %w", s, ErrInvalidDistance)
	}
	if symbol == "" {
		return 0, fmt.Errorf("parse distance %q: missing unit symbol: %w", s, ErrInvalidDistance)
	}
	var u U
	if symbol != u.Symbol() {
		return 0, fmt.Errorf("parse distance %q: want unit %s: %w", s, u.Symbol(), ErrUnitMismatch)
	}
	return Distance[U](f), nil
}

// Convert re-expresses d, measured in From, in unit To.
func Convert[To Unit, From Unit](d Distance[From]) Distance[To] {
	var from From
	var to To
	meters := float64(d) * from.MetersPerUnit()
	return Distance[To](meters / to.MetersPerUnit())
}

func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
