package typed

import "errors"

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
func (Meters) Symbol() string { return "" }

// MetersPerUnit returns 1.
func (Meters) MetersPerUnit() float64 { return 0 }

// Feet is the international foot. Symbol "ft", exactly 0.3048 m.
type Feet struct{}

// Symbol returns "ft".
func (Feet) Symbol() string { return "" }

// MetersPerUnit returns 0.3048.
func (Feet) MetersPerUnit() float64 { return 0 }

// Distance is a length measured in unit U. U is a phantom parameter: it does
// not appear in the underlying type, so Distance[Meters] and Distance[Feet]
// are distinct types with the same float64 representation.
type Distance[U Unit] float64

// Value returns the numeric magnitude in U.
func (d Distance[U]) Value() float64 { return 0 }

// Add returns d+other. Only distances in the same unit can be added.
func (d Distance[U]) Add(other Distance[U]) Distance[U] { return 0 }

// String returns the magnitude and symbol separated by one space, for example
// "12.5 m". The magnitude uses the shortest exact float64 form.
func (d Distance[U]) String() string { return "" }

// MarshalText returns the String form.
func (d Distance[U]) MarshalText() ([]byte, error) { return nil, nil }

// UnmarshalText parses text with ParseDistance. On error d is unchanged.
func (d *Distance[U]) UnmarshalText(text []byte) error { return nil }

// ParseDistance parses "<number>[ ]<symbol>" in unit U. Surrounding whitespace is
// ignored. The number must be finite. A symbol other than U.Symbol() yields
// ErrUnitMismatch; any other malformation yields ErrInvalidDistance.
func ParseDistance[U Unit](s string) (Distance[U], error) { return 0, nil }

// Convert re-expresses d, measured in From, in unit To.
func Convert[To Unit, From Unit](d Distance[From]) Distance[To] { return 0 }
