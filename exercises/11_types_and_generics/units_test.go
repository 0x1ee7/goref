package typed_test

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"

	"goref/exercises/11_types_and_generics"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestPart7UnitSymbolsAndFactors(t *testing.T) {
	if got := (typed.Meters{}).Symbol(); got != "m" {
		t.Errorf("Meters.Symbol() = %q, want m", got)
	}
	if got := (typed.Feet{}).Symbol(); got != "ft" {
		t.Errorf("Feet.Symbol() = %q, want ft", got)
	}
	if got := (typed.Meters{}).MetersPerUnit(); got != 1 {
		t.Errorf("Meters.MetersPerUnit() = %v, want 1", got)
	}
	if got := (typed.Feet{}).MetersPerUnit(); !approx(got, 0.3048) {
		t.Errorf("Feet.MetersPerUnit() = %v, want 0.3048", got)
	}
}

func TestPart7StringAndValue(t *testing.T) {
	m := typed.Distance[typed.Meters](12.5)
	if m.String() != "12.5 m" {
		t.Errorf("Meters(12.5).String() = %q, want %q", m.String(), "12.5 m")
	}
	if typed.Distance[typed.Feet](3).String() != "3 ft" {
		t.Errorf("Feet(3).String() = %q, want %q", typed.Distance[typed.Feet](3).String(), "3 ft")
	}
	if m.Value() != 12.5 {
		t.Errorf("Value() = %v, want 12.5", m.Value())
	}
	// Shortest exact float64 form, not a fixed precision.
	sum := typed.Distance[typed.Meters](0.1).Add(0.2)
	if got := sum.String(); got != "0.30000000000000004 m" {
		t.Errorf("0.1 m + 0.2 m String() = %q, want %q", got, "0.30000000000000004 m")
	}
}

func TestPart7Add(t *testing.T) {
	got := typed.Distance[typed.Feet](1.5).Add(2.25)
	if got.Value() != 3.75 {
		t.Errorf("1.5 ft + 2.25 ft = %v, want 3.75", got.Value())
	}
}

func TestPart7Convert(t *testing.T) {
	tests := []struct {
		name string
		got  float64
		want float64
	}{
		{"meters to feet", typed.Convert[typed.Feet](typed.Distance[typed.Meters](1)).Value(), 1 / 0.3048},
		{"feet to meters", typed.Convert[typed.Meters](typed.Distance[typed.Feet](10)).Value(), 3.048},
		{"same unit", typed.Convert[typed.Meters](typed.Distance[typed.Meters](2)).Value(), 2},
		{"round trip", typed.Convert[typed.Feet](typed.Convert[typed.Meters](typed.Distance[typed.Feet](7))).Value(), 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !approx(tt.got, tt.want) {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestPart7ParseDistance(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want float64
	}{
		{"spaced", "12.5 m", 12.5},
		{"unspaced", "12.5m", 12.5},
		{"exponent", "1e3 m", 1000},
		{"padded", "  5 m  ", 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := typed.ParseDistance[typed.Meters](tt.in)
			if err != nil || !approx(got.Value(), tt.want) {
				t.Errorf("ParseDistance[Meters](%q) = (%v, %v), want (%v, nil)", tt.in, got, err, tt.want)
			}
		})
	}
	if got, err := typed.ParseDistance[typed.Feet]("  4 ft "); err != nil || got.Value() != 4 {
		t.Errorf("ParseDistance[Feet](padded) = (%v, %v), want (4, nil)", got, err)
	}

	errs := []struct {
		name string
		in   string
		want error
	}{
		{"other unit", "4 ft", typed.ErrUnitMismatch},
		{"unknown unit", "4 yd", typed.ErrUnitMismatch},
		{"no unit", "4", typed.ErrInvalidDistance},
		{"not a number", "abc m", typed.ErrInvalidDistance},
		{"empty", "", typed.ErrInvalidDistance},
		{"infinite", "inf m", typed.ErrInvalidDistance},
		{"not a number literal", "NaN m", typed.ErrInvalidDistance},
	}
	for _, tt := range errs {
		t.Run(tt.name, func(t *testing.T) {
			_, err := typed.ParseDistance[typed.Meters](tt.in)
			if !errors.Is(err, tt.want) {
				t.Errorf("ParseDistance[Meters](%q) error = %v, want %v", tt.in, err, tt.want)
			}
		})
	}
}

func TestPart7StringRoundTrip(t *testing.T) {
	for _, v := range []float64{0, 0.1, -2.5, 1e-7, 123456.789} {
		d := typed.Distance[typed.Feet](v)
		got, err := typed.ParseDistance[typed.Feet](d.String())
		if err != nil || got != d {
			t.Errorf("round trip of %v via %q = (%v, %v), want %v", v, d.String(), got, err, d)
		}
	}
}

type shipment struct {
	Length typed.Distance[typed.Feet] `json:"length"`
}

func TestPart7TextAndJSON(t *testing.T) {
	b, err := typed.Distance[typed.Feet](3).MarshalText()
	if err != nil || string(b) != "3 ft" {
		t.Fatalf("MarshalText = (%q, %v), want (3 ft, nil)", b, err)
	}

	var d typed.Distance[typed.Feet]
	if err := d.UnmarshalText([]byte("2.5 ft")); err != nil || d.Value() != 2.5 {
		t.Fatalf("UnmarshalText(2.5 ft) -> %v, %v; want 2.5, nil", d, err)
	}
	if err := d.UnmarshalText([]byte("9 m")); !errors.Is(err, typed.ErrUnitMismatch) {
		t.Errorf("UnmarshalText(9 m) error = %v, want ErrUnitMismatch", err)
	}
	if d.Value() != 2.5 {
		t.Errorf("value after failed UnmarshalText = %v, want unchanged 2.5", d.Value())
	}

	out, err := json.Marshal(shipment{Length: 3})
	if err != nil || string(out) != `{"length":"3 ft"}` {
		t.Fatalf("json.Marshal = (%s, %v), want {\"length\":\"3 ft\"}", out, err)
	}
	var s shipment
	if err := json.Unmarshal([]byte(`{"length":"4.25 ft"}`), &s); err != nil || s.Length.Value() != 4.25 {
		t.Errorf("json.Unmarshal = (%+v, %v), want length 4.25 ft", s, err)
	}
	if err := json.Unmarshal([]byte(`{"length":"4 m"}`), &s); !errors.Is(err, typed.ErrUnitMismatch) {
		t.Errorf("json.Unmarshal of metres into feet error = %v, want ErrUnitMismatch", err)
	}
}

func TestPart7PhantomTypes(t *testing.T) {
	// Different phantom units are different types with the same representation.
	if reflect.TypeFor[typed.Distance[typed.Meters]]() == reflect.TypeFor[typed.Distance[typed.Feet]]() {
		t.Error("Distance[Meters] and Distance[Feet] are the same type")
	}
	if k := reflect.TypeFor[typed.Distance[typed.Feet]]().Kind(); k != reflect.Float64 {
		t.Errorf("Distance kind = %v, want float64", k)
	}
	m := typed.Distance[typed.Meters](5)
	if f := typed.Convert[typed.Feet](m); !approx(f.Value(), 5/0.3048) {
		t.Errorf("Convert(5 m) to feet = %v, want %v", f.Value(), 5/0.3048)
	}
}
