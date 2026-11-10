package structs_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"

	structs "goref/exercises/07_structs_and_methods"
)

// mustMoney builds a Money with NewMoney and fails the test on error.
func mustMoney(t *testing.T, minor int64, currency string) structs.Money {
	t.Helper()
	m, err := structs.NewMoney(minor, currency)
	if err != nil {
		t.Fatalf("NewMoney(%d, %q): unexpected error: %v", minor, currency, err)
	}
	return m
}

func minors(ms []structs.Money) []int64 {
	out := make([]int64, len(ms))
	for i, m := range ms {
		out[i] = m.Minor()
	}
	return out
}

func TestPart1NewMoney(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		tests := []struct {
			name     string
			minor    int64
			currency string
		}{
			{"usd cents", 1050, "USD"},
			{"yen has no minor units", 100, "JPY"},
			{"negative amount", -5, "EUR"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				m := mustMoney(t, tt.minor, tt.currency)
				if m.Minor() != tt.minor || m.Currency() != tt.currency {
					t.Errorf("got (%d, %q), want (%d, %q)", m.Minor(), m.Currency(), tt.minor, tt.currency)
				}
			})
		}
	})

	t.Run("invalid currency", func(t *testing.T) {
		for _, cur := range []string{"", "usd", "US", "USDX", "U5D", "US$", "ÄÖÜ"} {
			t.Run(fmt.Sprintf("%q", cur), func(t *testing.T) {
				_ = mustMoney(t, 1, "USD")
				if _, err := structs.NewMoney(1, cur); !errors.Is(err, structs.ErrInvalidCurrency) {
					t.Errorf("NewMoney(1, %q) error = %v, want ErrInvalidCurrency", cur, err)
				}
			})
		}
	})
}

func TestPart1ParseMoney(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		tests := []struct {
			amount, currency string
			want             int64
		}{
			{"10.50", "USD", 1050},
			{"10", "USD", 1000},
			{"0.5", "USD", 50},
			{"-0.05", "USD", -5},
			{"+3.00", "USD", 300},
			{"1.500", "USD", 150},
			{"-0.00", "USD", 0},
			{"100", "JPY", 100},
			{"1.005", "KWD", 1005},
			{"-92233720368547758.08", "USD", math.MinInt64},
		}
		for _, tt := range tests {
			t.Run(tt.amount+" "+tt.currency, func(t *testing.T) {
				_ = mustMoney(t, 1, "USD")
				m, err := structs.ParseMoney(tt.amount, tt.currency)
				if err != nil {
					t.Fatalf("ParseMoney(%q, %q): %v", tt.amount, tt.currency, err)
				}
				if m.Minor() != tt.want {
					t.Errorf("minor = %d, want %d", m.Minor(), tt.want)
				}
			})
		}
	})

	t.Run("precision", func(t *testing.T) {
		tests := []struct{ amount, currency string }{
			{"1.005", "USD"},
			{"1.5", "JPY"},
			{"0.0001", "KWD"},
		}
		for _, tt := range tests {
			t.Run(tt.amount+" "+tt.currency, func(t *testing.T) {
				_ = mustMoney(t, 1, "USD")
				if _, err := structs.ParseMoney(tt.amount, tt.currency); !errors.Is(err, structs.ErrPrecision) {
					t.Errorf("error = %v, want ErrPrecision", err)
				}
			})
		}
	})

	t.Run("malformed", func(t *testing.T) {
		for _, s := range []string{"", "abc", "1e3", "5.", ".5", " 1.00", "1.00 ", "1,00", "NaN", "Inf", "--1", "1.2.3"} {
			t.Run(fmt.Sprintf("%q", s), func(t *testing.T) {
				_ = mustMoney(t, 1, "USD")
				if _, err := structs.ParseMoney(s, "USD"); !errors.Is(err, structs.ErrInvalidAmount) {
					t.Errorf("ParseMoney(%q) error = %v, want ErrInvalidAmount", s, err)
				}
			})
		}
	})

	t.Run("overflow", func(t *testing.T) {
		for _, s := range []string{"92233720368547758.08", "99999999999999999999"} {
			t.Run(s, func(t *testing.T) {
				_ = mustMoney(t, 1, "USD")
				if _, err := structs.ParseMoney(s, "USD"); !errors.Is(err, structs.ErrOverflow) {
					t.Errorf("ParseMoney(%q) error = %v, want ErrOverflow", s, err)
				}
			})
		}
	})

	t.Run("invalid currency", func(t *testing.T) {
		_ = mustMoney(t, 1, "USD")
		if _, err := structs.ParseMoney("1.00", "usd"); !errors.Is(err, structs.ErrInvalidCurrency) {
			t.Errorf("error = %v, want ErrInvalidCurrency", err)
		}
	})
}

func TestPart1ZeroValue(t *testing.T) {
	t.Run("zero value has no currency and is zero", func(t *testing.T) {
		_ = mustMoney(t, 1, "USD")
		var zero structs.Money
		if zero.Currency() != "" || !zero.IsZero() {
			t.Errorf("zero Money: currency %q, IsZero %v; want \"\", true", zero.Currency(), zero.IsZero())
		}
	})

	t.Run("zero value prints as bare 0.00", func(t *testing.T) {
		_ = mustMoney(t, 1, "USD")
		var zero structs.Money
		if got := zero.String(); got != "0.00" {
			t.Errorf("String() = %q, want %q", got, "0.00")
		}
	})

	t.Run("zero value does not mix with real currency", func(t *testing.T) {
		usd := mustMoney(t, 100, "USD")
		var zero structs.Money
		if _, err := zero.Add(usd); !errors.Is(err, structs.ErrCurrencyMismatch) {
			t.Errorf("zero.Add(USD) error = %v, want ErrCurrencyMismatch", err)
		}
	})

	t.Run("zero value cannot be marshalled", func(t *testing.T) {
		_ = mustMoney(t, 1, "USD")
		var zero structs.Money
		if _, err := zero.MarshalText(); !errors.Is(err, structs.ErrInvalidCurrency) {
			t.Errorf("MarshalText error = %v, want ErrInvalidCurrency", err)
		}
	})
}

func TestPart1Arithmetic(t *testing.T) {
	t.Run("add", func(t *testing.T) {
		usd := mustMoney(t, 1050, "USD")
		sum, err := usd.Add(mustMoney(t, -50, "USD"))
		if err != nil {
			t.Fatalf("Add: %v", err)
		}
		if sum.Minor() != 1000 || sum.Currency() != "USD" {
			t.Errorf("sum = %d %s, want 1000 USD", sum.Minor(), sum.Currency())
		}
	})

	t.Run("add mismatch", func(t *testing.T) {
		usd := mustMoney(t, 1050, "USD")
		if _, err := usd.Add(mustMoney(t, 1, "EUR")); !errors.Is(err, structs.ErrCurrencyMismatch) {
			t.Errorf("error = %v, want ErrCurrencyMismatch", err)
		}
	})

	t.Run("add overflow", func(t *testing.T) {
		max := mustMoney(t, math.MaxInt64, "USD")
		if _, err := max.Add(mustMoney(t, 1, "USD")); !errors.Is(err, structs.ErrOverflow) {
			t.Errorf("MaxInt64 + 1 error = %v, want ErrOverflow", err)
		}
		min := mustMoney(t, math.MinInt64, "USD")
		if _, err := min.Add(mustMoney(t, -1, "USD")); !errors.Is(err, structs.ErrOverflow) {
			t.Errorf("MinInt64 + -1 error = %v, want ErrOverflow", err)
		}
	})

	t.Run("sub", func(t *testing.T) {
		usd := mustMoney(t, 100, "USD")
		diff, err := usd.Sub(mustMoney(t, 250, "USD"))
		if err != nil {
			t.Fatalf("Sub: %v", err)
		}
		if diff.Minor() != -150 {
			t.Errorf("Sub = %d, want -150", diff.Minor())
		}
	})

	t.Run("sub overflow and mismatch", func(t *testing.T) {
		min := mustMoney(t, math.MinInt64, "USD")
		if _, err := min.Sub(mustMoney(t, 1, "USD")); !errors.Is(err, structs.ErrOverflow) {
			t.Errorf("MinInt64 - 1 error = %v, want ErrOverflow", err)
		}
		usd := mustMoney(t, 100, "USD")
		if _, err := usd.Sub(mustMoney(t, 1, "GBP")); !errors.Is(err, structs.ErrCurrencyMismatch) {
			t.Errorf("error = %v, want ErrCurrencyMismatch", err)
		}
	})

	t.Run("compare", func(t *testing.T) {
		tests := []struct {
			a, b int64
			want int
		}{
			{1, 2, -1},
			{2, 2, 0},
			{3, 2, 1},
		}
		for _, tt := range tests {
			a := mustMoney(t, tt.a, "USD")
			got, err := a.Compare(mustMoney(t, tt.b, "USD"))
			if err != nil {
				t.Fatalf("Compare(%d, %d): %v", tt.a, tt.b, err)
			}
			if got != tt.want {
				t.Errorf("Compare(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		}
	})

	t.Run("compare mismatch", func(t *testing.T) {
		usd := mustMoney(t, 1, "USD")
		if _, err := usd.Compare(mustMoney(t, 1, "JPY")); !errors.Is(err, structs.ErrCurrencyMismatch) {
			t.Errorf("error = %v, want ErrCurrencyMismatch", err)
		}
	})
}

func TestPart1Scale(t *testing.T) {
	tests := []struct {
		name     string
		minor    int64
		num, den int64
		want     int64
	}{
		{"exact", 1050, 3, 2, 1575},
		{"third rounds down", 1000, 1, 3, 333},
		{"half to even down", 5, 1, 2, 2},
		{"half to even up", 7, 1, 2, 4},
		{"negative half", -5, 1, 2, -2},
		{"tenth tie to even", 15, 1, 10, 2},
		{"tenth tie up", 35, 1, 10, 4},
		{"negative denominator", 5, 1, -2, -2},
		{"zero", 0, 7, 3, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := mustMoney(t, tt.minor, "USD")
			got, err := m.Scale(tt.num, tt.den)
			if err != nil {
				t.Fatalf("Scale(%d, %d): %v", tt.num, tt.den, err)
			}
			if got.Minor() != tt.want || got.Currency() != "USD" {
				t.Errorf("Scale = %d %s, want %d USD", got.Minor(), got.Currency(), tt.want)
			}
		})
	}

	t.Run("zero denominator", func(t *testing.T) {
		m := mustMoney(t, 5, "USD")
		if _, err := m.Scale(1, 0); !errors.Is(err, structs.ErrInvalidArgument) {
			t.Errorf("error = %v, want ErrInvalidArgument", err)
		}
	})

	t.Run("overflow", func(t *testing.T) {
		m := mustMoney(t, math.MaxInt64, "USD")
		if _, err := m.Scale(2, 1); !errors.Is(err, structs.ErrOverflow) {
			t.Errorf("error = %v, want ErrOverflow", err)
		}
	})
}

func TestPart1Allocate(t *testing.T) {
	tests := []struct {
		name  string
		minor int64
		n     int
		want  []int64
	}{
		{"thirds of a dollar", 1000, 3, []int64{334, 333, 333}},
		{"negative thirds", -1000, 3, []int64{-334, -333, -333}},
		{"uneven halves", 5, 2, []int64{3, 2}},
		{"fewer units than shares", 1, 3, []int64{1, 0, 0}},
		{"single share", 99, 1, []int64{99}},
		{"yen into three", 10, 3, []int64{4, 3, 3}},
		{"min int64 single share", math.MinInt64, 1, []int64{math.MinInt64}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := mustMoney(t, tt.minor, "USD")
			shares, err := m.Allocate(tt.n)
			if err != nil {
				t.Fatalf("Allocate(%d): %v", tt.n, err)
			}
			if diff := cmp.Diff(tt.want, minors(shares)); diff != "" {
				t.Errorf("shares mismatch (-want +got):\n%s", diff)
			}
			for _, s := range shares {
				if s.Currency() != "USD" {
					t.Errorf("share currency = %q, want USD", s.Currency())
				}
			}
		})
	}

	t.Run("invalid share count", func(t *testing.T) {
		m := mustMoney(t, 100, "USD")
		for _, n := range []int{0, -1} {
			if _, err := m.Allocate(n); !errors.Is(err, structs.ErrInvalidArgument) {
				t.Errorf("Allocate(%d) error = %v, want ErrInvalidArgument", n, err)
			}
		}
	})
}

func TestPart1Equality(t *testing.T) {
	t.Run("== compares value and currency", func(t *testing.T) {
		a := mustMoney(t, 100, "USD")
		if a != mustMoney(t, 100, "USD") {
			t.Errorf("equal amounts compare unequal")
		}
		if a == mustMoney(t, 100, "EUR") {
			t.Errorf("different currencies compare equal")
		}
	})

	t.Run("usable as map key", func(t *testing.T) {
		a := mustMoney(t, 100, "USD")
		counts := map[structs.Money]int{}
		counts[a]++
		counts[mustMoney(t, 100, "USD")]++
		if len(counts) != 1 || counts[a] != 2 {
			t.Errorf("counts = %v, want one key with count 2", counts)
		}
	})
}

func TestPart1Format(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		tests := []struct {
			minor    int64
			currency string
			want     string
		}{
			{1050, "USD", "10.50 USD"},
			{-5, "USD", "-0.05 USD"},
			{1, "USD", "0.01 USD"},
			{100, "JPY", "100 JPY"},
			{1005, "KWD", "1.005 KWD"},
		}
		for _, tt := range tests {
			m := mustMoney(t, tt.minor, tt.currency)
			if got := m.String(); got != tt.want {
				t.Errorf("String(%d %s) = %q, want %q", tt.minor, tt.currency, got, tt.want)
			}
		}
	})

	t.Run("verbs", func(t *testing.T) {
		m := mustMoney(t, 1050, "USD")
		tests := []struct {
			name, got, want string
		}{
			{"%v", fmt.Sprintf("%v", m), "10.50 USD"},
			{"%s", fmt.Sprintf("%s", m), "10.50 USD"},
			{"%12v", fmt.Sprintf("%12v|", m), "   10.50 USD|"},
			{"%-12s", fmt.Sprintf("%-12s|", m), "10.50 USD   |"},
			{"%d", fmt.Sprintf("%d", m), "1050"},
			{"%#v", fmt.Sprintf("%#v", m), `structs.NewMoney(1050, "USD")`},
			{"%x unsupported", fmt.Sprintf("%x", m), "%!x(structs.Money=10.50 USD)"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if tt.got != tt.want {
					t.Errorf("got %q, want %q", tt.got, tt.want)
				}
			})
		}
	})

	t.Run("GoString", func(t *testing.T) {
		m := mustMoney(t, 1050, "USD")
		if got := m.GoString(); got != `structs.NewMoney(1050, "USD")` {
			t.Errorf("GoString() = %q", got)
		}
		var zero structs.Money
		if got := zero.GoString(); got != "structs.Money{}" {
			t.Errorf("zero GoString() = %q, want structs.Money{}", got)
		}
	})
}

func TestPart1Text(t *testing.T) {
	t.Run("MarshalText", func(t *testing.T) {
		m := mustMoney(t, 1050, "USD")
		got, err := m.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText: %v", err)
		}
		if string(got) != "10.50 USD" {
			t.Errorf("MarshalText = %q, want %q", got, "10.50 USD")
		}
	})

	t.Run("UnmarshalText round trip", func(t *testing.T) {
		var m structs.Money
		if err := m.UnmarshalText([]byte("-0.05 USD")); err != nil {
			t.Fatalf("UnmarshalText: %v", err)
		}
		if m != mustMoney(t, -5, "USD") {
			t.Errorf("got %v, want -0.05 USD", m)
		}
	})

	t.Run("UnmarshalText error leaves receiver unchanged", func(t *testing.T) {
		m := mustMoney(t, 7, "EUR")
		if err := m.UnmarshalText([]byte("10.50")); !errors.Is(err, structs.ErrInvalidAmount) {
			t.Errorf("error = %v, want ErrInvalidAmount", err)
		}
		if err := m.UnmarshalText([]byte("1.005 USD")); !errors.Is(err, structs.ErrPrecision) {
			t.Errorf("error = %v, want ErrPrecision", err)
		}
		if m != mustMoney(t, 7, "EUR") {
			t.Errorf("receiver changed to %v after failed unmarshal", m)
		}
	})

	t.Run("JSON struct field", func(t *testing.T) {
		type invoice struct {
			Total structs.Money `json:"total"`
		}
		in := invoice{Total: mustMoney(t, 1050, "USD")}
		raw, err := json.Marshal(in)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if string(raw) != `{"total":"10.50 USD"}` {
			t.Errorf("json = %s", raw)
		}
		var out invoice
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if out != in {
			t.Errorf("round trip = %v, want %v", out, in)
		}
	})

	t.Run("JSON map key", func(t *testing.T) {
		in := map[structs.Money]int{mustMoney(t, 1050, "USD"): 3}
		raw, err := json.Marshal(in)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if string(raw) != `{"10.50 USD":3}` {
			t.Errorf("json = %s", raw)
		}
		var out map[structs.Money]int
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if out[mustMoney(t, 1050, "USD")] != 3 {
			t.Errorf("decoded map = %v", out)
		}
	})
}
