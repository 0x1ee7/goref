package typed_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"goref/exercises/11_types_and_generics"
)

// Cents is a defined type; it satisfies Number only because of the ~ in the
// constraint's type set.
type Cents int64

var errBad = errors.New("bad input")

func TestPart3Sum(t *testing.T) {
	if got := typed.Sum([]int{1, 2, 3}); got != 6 {
		t.Errorf("Sum(int) = %d, want 6", got)
	}
	if got := typed.Sum([]float64{0.5, 0.25}); got != 0.75 {
		t.Errorf("Sum(float64) = %v, want 0.75", got)
	}
	if got := typed.Sum([]Cents{150, 275}); got != 425 {
		t.Errorf("Sum(Cents) = %d, want 425", got)
	}
	if got := typed.Sum([]int32{}); got != 0 {
		t.Errorf("Sum(empty) = %d, want 0", got)
	}
}

func TestPart3Max(t *testing.T) {
	got, err := typed.Max([]int{3, 9, 2})
	if err != nil || got != 9 {
		t.Errorf("Max([3 9 2]) = (%d, %v), want (9, nil)", got, err)
	}
	s, err := typed.Max([]string{"pear", "apple"})
	if err != nil || s != "pear" {
		t.Errorf("Max(strings) = (%q, %v), want (pear, nil)", s, err)
	}
	c, err := typed.Max([]Cents{5, 40, 7})
	if err != nil || c != 40 {
		t.Errorf("Max(Cents) = (%d, %v), want (40, nil)", c, err)
	}
	if _, err := typed.Max([]int{}); !errors.Is(err, typed.ErrEmpty) {
		t.Errorf("Max(empty) error = %v, want ErrEmpty", err)
	}
}

func TestPart3Clamp(t *testing.T) {
	tests := []struct {
		name   string
		v, lo  int
		hi     int
		want   int
		wantEr error
	}{
		{"inside", 5, 1, 10, 5, nil},
		{"below", -3, 1, 10, 1, nil},
		{"above", 42, 0, 10, 10, nil},
		{"bounds inverted", 5, 10, 1, 0, typed.ErrBounds},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := typed.Clamp(tt.v, tt.lo, tt.hi)
			if !errors.Is(err, tt.wantEr) {
				t.Fatalf("Clamp(%d,%d,%d) error = %v, want %v", tt.v, tt.lo, tt.hi, err, tt.wantEr)
			}
			if got != tt.want {
				t.Errorf("Clamp(%d,%d,%d) = %d, want %d", tt.v, tt.lo, tt.hi, got, tt.want)
			}
		})
	}
	if got, err := typed.Clamp("m", "a", "k"); err != nil || got != "k" {
		t.Errorf("Clamp(strings) = (%q, %v), want (k, nil)", got, err)
	}
}

func TestPart3GroupBy(t *testing.T) {
	words := []string{"apple", "banana", "avocado", "blueberry", "cherry"}
	got := typed.GroupBy(words, func(s string) string { return s[:1] })
	want := map[string][]string{
		"a": {"apple", "avocado"},
		"b": {"banana", "blueberry"},
		"c": {"cherry"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("GroupBy mismatch (-want +got):\n%s", diff)
	}

	empty := typed.GroupBy([]string{}, func(s string) string { return s })
	if empty == nil || len(empty) != 0 {
		t.Errorf("GroupBy(empty) = %#v, want non-nil empty map", empty)
	}
}

func TestPart3SortedKeys(t *testing.T) {
	got := typed.SortedKeys(map[string]int{"b": 1, "a": 2, "c": 3})
	if diff := cmp.Diff([]string{"a", "b", "c"}, got); diff != "" {
		t.Errorf("SortedKeys mismatch (-want +got):\n%s", diff)
	}
	ints := typed.SortedKeys(map[int]bool{10: true, -1: false, 3: true})
	if diff := cmp.Diff([]int{-1, 3, 10}, ints); diff != "" {
		t.Errorf("SortedKeys(int) mismatch (-want +got):\n%s", diff)
	}
	none := typed.SortedKeys[string, int](nil)
	if none == nil || len(none) != 0 {
		t.Errorf("SortedKeys(nil) = %#v, want non-nil empty slice", none)
	}
}

func TestPart3ParsePort(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    typed.Port
		wantErr bool
	}{
		{"http", "8080", 8080, false},
		{"padded", " 443 ", 443, false},
		{"lowest", "1", 1, false},
		{"highest", "65535", 65535, false},
		{"zero", "0", 0, true},
		{"too large", "65536", 0, true},
		{"not a number", "http", 0, true},
		{"empty", "", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := typed.ParsePort(tt.in)
			if tt.wantErr {
				if !errors.Is(err, typed.ErrInvalidPort) {
					t.Fatalf("ParsePort(%q) error = %v, want ErrInvalidPort", tt.in, err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("ParsePort(%q) = (%d, %v), want (%d, nil)", tt.in, got, err, tt.want)
			}
		})
	}
}

func parseString(s string) (string, error) { return s, nil }

func TestPart3SettingRequired(t *testing.T) {
	env := map[string]string{
		"DB_HOST": "  db.internal  ",
		"BLANK":   "   ",
		"BAD_PORT": "zzz",
	}
	host := typed.Required("DB_HOST", parseString)
	if host.Key() != "DB_HOST" {
		t.Errorf("Key() = %q, want DB_HOST", host.Key())
	}
	got, err := host.Load(env)
	if err != nil || got != "db.internal" {
		t.Errorf("Load(DB_HOST) = (%q, %v), want (db.internal, nil): value must be trimmed before parse", got, err)
	}

	for _, key := range []string{"MISSING", "BLANK"} {
		_, err := typed.Required(key, parseString).Load(env)
		if !errors.Is(err, typed.ErrMissing) {
			t.Errorf("Load(%s) error = %v, want ErrMissing", key, err)
		}
		if err != nil && !strings.Contains(err.Error(), key) {
			t.Errorf("Load(%s) error %q does not name the key", key, err)
		}
	}

	port := typed.Required("BAD_PORT", strconv.Atoi)
	_, err = port.Load(env)
	if err == nil || !strings.Contains(err.Error(), "BAD_PORT") {
		t.Errorf("Load(BAD_PORT) error = %v, want an error naming BAD_PORT", err)
	}
	failing := typed.Required("X", func(string) (string, error) { return "", errBad })
	_, err = failing.Load(map[string]string{"X": "v"})
	if !errors.Is(err, errBad) {
		t.Errorf("parse failure error = %v, want it to wrap the parser's error", err)
	}
}

func TestPart3SettingWithDefault(t *testing.T) {
	env := map[string]string{"TIMEOUT": "45", "RETRIES": "x", "EMPTY": ""}
	timeout := typed.WithDefault("TIMEOUT", 30, strconv.Atoi)
	if got, err := timeout.Load(env); err != nil || got != 45 {
		t.Errorf("Load(TIMEOUT=45) = (%d, %v), want (45, nil)", got, err)
	}
	if got, err := typed.WithDefault("ABSENT", 30, strconv.Atoi).Load(env); err != nil || got != 30 {
		t.Errorf("Load(absent) = (%d, %v), want (30, nil)", got, err)
	}
	if got, err := typed.WithDefault("EMPTY", 30, strconv.Atoi).Load(env); err != nil || got != 30 {
		t.Errorf("Load(blank) = (%d, %v), want (30, nil)", got, err)
	}
	// A present but invalid value is an error, never silently replaced.
	if _, err := typed.WithDefault("RETRIES", 3, strconv.Atoi).Load(env); err == nil {
		t.Error("Load(RETRIES=x) error = nil, want parse error")
	}
	if got := typed.WithDefault("ABSENT", 1, strconv.Atoi).Key(); got != "ABSENT" {
		t.Errorf("Key() = %q, want ABSENT", got)
	}
}
