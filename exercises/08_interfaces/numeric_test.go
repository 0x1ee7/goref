package interfaces_test

import (
	"testing"

	"goref/exercises/08_interfaces"
)

type cents int

type ratio float64

func TestPart7GenericSum(t *testing.T) {
	t.Run("sums named types whose underlying type is in the type set", func(t *testing.T) {
		if got := interfaces.Sum([]cents{1, 2, 3}); got != 6 {
			t.Errorf("Sum(cents) = %d, want 6", got)
		}
		if got := interfaces.Sum([]ratio{0.5, 0.25}); got != 0.75 {
			t.Errorf("Sum(ratio) = %v, want 0.75", got)
		}
	})

	t.Run("empty sum is the zero value", func(t *testing.T) {
		if got := interfaces.Sum([]int{4, 5}); got != 9 {
			t.Fatalf("sanity: Sum([4 5]) = %d, want 9", got)
		}
		if got := interfaces.Sum([]int(nil)); got != 0 {
			t.Errorf("Sum(nil) = %d, want 0", got)
		}
	})
}

func TestPart7GenericMax(t *testing.T) {
	t.Run("max of ordered strings and ints", func(t *testing.T) {
		if got, ok := interfaces.Max([]string{"b", "c", "a"}); !ok || got != "c" {
			t.Errorf("Max(strings) = %q, %v; want c, true", got, ok)
		}
		if got, ok := interfaces.Max([]int{-3, 7, 2}); !ok || got != 7 {
			t.Errorf("Max(ints) = %d, %v; want 7, true", got, ok)
		}
	})

	t.Run("empty input reports false", func(t *testing.T) {
		if got, ok := interfaces.Max([]int{1}); !ok || got != 1 {
			t.Fatalf("sanity: Max([1]) = %d, %v; want 1, true", got, ok)
		}
		if got, ok := interfaces.Max([]int(nil)); ok || got != 0 {
			t.Errorf("Max(nil) = %d, %v; want 0, false", got, ok)
		}
	})
}

func TestPart7GenericJoin(t *testing.T) {
	t.Run("renders items through their String methods", func(t *testing.T) {
		sevs := []interfaces.Severity{interfaces.SeverityInfo, interfaces.SeverityError}
		if got := interfaces.Join(sevs, ", "); got != "info, error" {
			t.Errorf("Join = %q, want %q", got, "info, error")
		}
	})

	t.Run("empty input joins to the empty string", func(t *testing.T) {
		if got := interfaces.Join([]interfaces.Severity{interfaces.SeverityWarning}, "|"); got != "warning" {
			t.Fatalf("sanity: Join single = %q, want %q", got, "warning")
		}
		if got := interfaces.Join[interfaces.Severity](nil, ","); got != "" {
			t.Errorf("Join(nil) = %q, want empty", got)
		}
	})
}
