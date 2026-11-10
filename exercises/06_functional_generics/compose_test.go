package functional_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	fp "goref/exercises/06_functional_generics"
)

func TestPart2Compose(t *testing.T) {
	length := func(s string) int { return len(s) }
	times10 := func(n int) int { return n * 10 }
	t.Run("applies g first, then f", func(t *testing.T) {
		if got := fp.Compose(times10, length)("hello"); got != 50 {
			t.Errorf("Compose(times10, length)(\"hello\") = %d, want 50", got)
		}
	})
	t.Run("may change type between stages", func(t *testing.T) {
		if got := fp.Compose(strconv.Itoa, times10)(4); got != "40" {
			t.Errorf("Compose(strconv.Itoa, times10)(4) = %q, want \"40\"", got)
		}
	})
}

func TestPart2Pipe(t *testing.T) {
	inc := func(n int) int { return n + 1 }
	dbl := func(n int) int { return n * 2 }
	tests := []struct {
		name   string
		v      int
		stages []func(int) int
		want   int
	}{
		{"left to right", 3, []func(int) int{inc, dbl}, 8},
		{"order matters", 3, []func(int) int{dbl, inc}, 7},
		{"no stages returns input", 7, nil, 7},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := fp.Pipe(tc.v, tc.stages...); got != tc.want {
				t.Errorf("Pipe(%d, ...) = %d, want %d", tc.v, got, tc.want)
			}
		})
	}
}

func TestPart2Curry(t *testing.T) {
	add := func(a, b int) int { return a + b }
	t.Run("Curry2 applies one argument at a time", func(t *testing.T) {
		if got := fp.Curry2(add)(2)(3); got != 5 {
			t.Errorf("Curry2(add)(2)(3) = %d, want 5", got)
		}
	})
	t.Run("Curry2 partial applications are independent", func(t *testing.T) {
		add5 := fp.Curry2(add)(5)
		a, b := add5(1), add5(2)
		if a != 6 || b != 7 {
			t.Errorf("add5(1), add5(2) = %d, %d; want 6, 7", a, b)
		}
		if got := fp.Curry2(add)(10)(1); got != 11 {
			t.Errorf("Curry2(add)(10)(1) = %d, want 11", got)
		}
	})
	t.Run("Curry3 threads three mixed-type arguments", func(t *testing.T) {
		join := func(a int, b string, c bool) string { return fmt.Sprintf("%d-%s-%t", a, b, c) }
		if got := fp.Curry3(join)(1)("x")(true); got != "1-x-true" {
			t.Errorf("Curry3(join)(1)(\"x\")(true) = %q, want \"1-x-true\"", got)
		}
	})
}

func TestPart2Flip(t *testing.T) {
	t.Run("swaps the two arguments", func(t *testing.T) {
		sub := func(a, b int) int { return a - b }
		if got := fp.Flip(sub)(1, 10); got != 9 {
			t.Errorf("Flip(sub)(1, 10) = %d, want 9", got)
		}
	})
	t.Run("swaps arguments of different types", func(t *testing.T) {
		rep := func(s string, n int) string { return strings.Repeat(s, n) }
		if got := fp.Flip(rep)(3, "ab"); got != "ababab" {
			t.Errorf("Flip(rep)(3, \"ab\") = %q, want \"ababab\"", got)
		}
	})
}

func TestPart2Bind(t *testing.T) {
	t.Run("fixes the leading argument", func(t *testing.T) {
		if got := fp.Bind(strings.Repeat, "ab")(3); got != "ababab" {
			t.Errorf("Bind(strings.Repeat, \"ab\")(3) = %q, want \"ababab\"", got)
		}
	})
	t.Run("bound closures are reusable", func(t *testing.T) {
		x := fp.Bind(strings.Repeat, "x")
		if a, b := x(2), x(4); a != "xx" || b != "xxxx" {
			t.Errorf("Bind reuse = %q, %q; want \"xx\", \"xxxx\"", a, b)
		}
	})
}
