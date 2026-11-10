package functional_test

import (
	"errors"
	"iter"
	"slices"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"

	fp "goref/exercises/06_functional_generics"
)

var errBoom = errors.New("boom")

func TestPart5Result(t *testing.T) {
	t.Run("Ok unwraps to its value", func(t *testing.T) {
		v, err := fp.Ok(3).Unwrap()
		if v != 3 || err != nil {
			t.Errorf("Ok(3).Unwrap() = (%d, %v), want (3, <nil>)", v, err)
		}
		if !fp.Ok(3).IsOK() {
			t.Error("Ok(3).IsOK() = false, want true")
		}
	})
	t.Run("Err unwraps to its error", func(t *testing.T) {
		v, err := fp.Err[int](errBoom).Unwrap()
		if v != 0 || !errors.Is(err, errBoom) {
			t.Errorf("Err(boom).Unwrap() = (%d, %v), want (0, boom)", v, err)
		}
		if fp.Err[int](errBoom).IsOK() {
			t.Error("Err(boom).IsOK() = true, want false")
		}
	})
	t.Run("OrElse picks the value or the fallback", func(t *testing.T) {
		if got := fp.Ok(5).OrElse(9); got != 5 {
			t.Errorf("Ok(5).OrElse(9) = %d, want 5", got)
		}
		if got := fp.Err[int](errBoom).OrElse(9); got != 9 {
			t.Errorf("Err.OrElse(9) = %d, want 9", got)
		}
	})
	t.Run("MapResult transforms ok and skips err", func(t *testing.T) {
		double := func(n int) int { return n * 2 }
		v, err := fp.MapResult(fp.Ok(2), double).Unwrap()
		if v != 4 || err != nil {
			t.Errorf("MapResult(Ok(2)) = (%d, %v), want (4, <nil>)", v, err)
		}
		calls := 0
		_, err = fp.MapResult(fp.Err[int](errBoom), func(n int) int { calls++; return n }).Unwrap()
		if !errors.Is(err, errBoom) || calls != 0 {
			t.Errorf("MapResult(Err) err = %v, calls = %d; want boom and 0 calls", err, calls)
		}
	})
	t.Run("BindResult chains and short-circuits", func(t *testing.T) {
		parse := fp.Lift(strconv.Atoi)
		v, err := fp.BindResult(fp.Ok("42"), parse).Unwrap()
		if v != 42 || err != nil {
			t.Errorf("BindResult(Ok(\"42\")) = (%d, %v), want (42, <nil>)", v, err)
		}
		if _, err := fp.BindResult(fp.Ok("x"), parse).Unwrap(); err == nil {
			t.Error("BindResult(Ok(\"x\")) err = nil, want a parse error")
		}
		calls := 0
		_, err = fp.BindResult(fp.Err[string](errBoom), func(s string) fp.Result[int] {
			calls++
			return fp.Ok(1)
		}).Unwrap()
		if !errors.Is(err, errBoom) || calls != 0 {
			t.Errorf("BindResult(Err) err = %v, calls = %d; want boom and 0 calls", err, calls)
		}
	})
	t.Run("Lift adapts (U, error) functions", func(t *testing.T) {
		v, err := fp.Lift(strconv.Atoi)("7").Unwrap()
		if v != 7 || err != nil {
			t.Errorf("Lift(Atoi)(\"7\") = (%d, %v), want (7, <nil>)", v, err)
		}
		if _, err := fp.Lift(strconv.Atoi)("seven").Unwrap(); err == nil {
			t.Error("Lift(Atoi)(\"seven\") err = nil, want an error")
		}
	})
}

func TestPart5CollectResults(t *testing.T) {
	t.Run("all ok returns values in order", func(t *testing.T) {
		rs := slices.Values([]fp.Result[int]{fp.Ok(1), fp.Ok(2), fp.Ok(3)})
		got, err := fp.CollectResults(rs)
		if err != nil {
			t.Fatalf("CollectResults error: %v", err)
		}
		if diff := cmp.Diff([]int{1, 2, 3}, got); diff != "" {
			t.Errorf("CollectResults mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("stops consuming at the first error", func(t *testing.T) {
		pulled := 0
		var rs iter.Seq[fp.Result[int]] = func(yield func(fp.Result[int]) bool) {
			for _, r := range []fp.Result[int]{fp.Ok(1), fp.Err[int](errBoom), fp.Ok(3)} {
				pulled++
				if !yield(r) {
					return
				}
			}
		}
		_, err := fp.CollectResults(rs)
		if !errors.Is(err, errBoom) {
			t.Errorf("CollectResults err = %v, want boom", err)
		}
		if pulled != 2 {
			t.Errorf("source pulled %d items, want 2 (stop at first error)", pulled)
		}
	})
	t.Run("empty input yields non-nil empty slice", func(t *testing.T) {
		got, err := fp.CollectResults(slices.Values([]fp.Result[int]{}))
		if err != nil {
			t.Fatalf("CollectResults error: %v", err)
		}
		if got == nil || len(got) != 0 {
			t.Errorf("CollectResults(empty) = %v, want non-nil empty slice", got)
		}
	})
}

func TestPart5ParsePort(t *testing.T) {
	const rangeMsg = "port must be between 1 and 65535"
	tests := []struct {
		name    string
		raw     string
		want    int
		wantErr string
	}{
		{"plain", "8080", 8080, ""},
		{"trims whitespace", " 443 ", 443, ""},
		{"lower bound", "1", 1, ""},
		{"upper bound", "65535", 65535, ""},
		{"zero is out of range", "0", 0, rangeMsg},
		{"too large", "65536", 0, rangeMsg},
		{"overflowing integer", "99999999999999999999", 0, rangeMsg},
		{"not a number", "http", 0, `port must be an integer, got "http"`},
		{"empty", "", 0, `port must be an integer, got ""`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := fp.ParsePort(tc.raw)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Errorf("ParsePort(%q) err = %v, want %q", tc.raw, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePort(%q) error: %v", tc.raw, err)
			}
			if got != tc.want {
				t.Errorf("ParsePort(%q) = %d, want %d", tc.raw, got, tc.want)
			}
		})
	}
}

func TestPart5ParsePorts(t *testing.T) {
	t.Run("parses a comma-separated list", func(t *testing.T) {
		got, err := fp.ParsePorts("80, 443,8080")
		if err != nil {
			t.Fatalf("ParsePorts error: %v", err)
		}
		if diff := cmp.Diff([]int{80, 443, 8080}, got); diff != "" {
			t.Errorf("ParsePorts mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("single port", func(t *testing.T) {
		got, err := fp.ParsePorts("22")
		if err != nil {
			t.Fatalf("ParsePorts error: %v", err)
		}
		if diff := cmp.Diff([]int{22}, got); diff != "" {
			t.Errorf("ParsePorts mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("first bad entry is the error", func(t *testing.T) {
		_, err := fp.ParsePorts("80,x,99999")
		if err == nil || err.Error() != `port must be an integer, got "x"` {
			t.Errorf("ParsePorts err = %v, want the error for \"x\"", err)
		}
	})
}

func TestPart5ParseBool(t *testing.T) {
	t.Run("true-like words", func(t *testing.T) {
		for _, raw := range []string{"true", "YES", "1", " On "} {
			got, err := fp.ParseBool(raw)
			if err != nil || got != true {
				t.Errorf("ParseBool(%q) = (%v, %v), want (true, <nil>)", raw, got, err)
			}
		}
	})
	t.Run("false-like words", func(t *testing.T) {
		for _, raw := range []string{"false", "No", "0", "off"} {
			got, err := fp.ParseBool(raw)
			if err != nil || got != false {
				t.Errorf("ParseBool(%q) = (%v, %v), want (false, <nil>)", raw, got, err)
			}
		}
		// Control: a false result above is only meaningful if "on" really parses as true.
		if got, err := fp.ParseBool("on"); err != nil || !got {
			t.Errorf("control ParseBool(\"on\") = (%v, %v), want (true, <nil>)", got, err)
		}
	})
	t.Run("unknown word is an error", func(t *testing.T) {
		_, err := fp.ParseBool("maybe")
		if err == nil || err.Error() != "expected boolean, got 'maybe'" {
			t.Errorf("ParseBool(\"maybe\") err = %v, want \"expected boolean, got 'maybe'\"", err)
		}
	})
}
