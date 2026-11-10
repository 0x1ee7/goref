package scope_test

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"goref/exercises/03_packages_and_scope"
)

var errBoom = errors.New("boom")

// formulaCells is shared read-only input for the evaluator tests.
var formulaCells = map[string]float64{"A1": 4, "B2": 2.5}

// mustDefault returns a built-in function or fails the test.
func mustDefault(t *testing.T, name string) scope.Func {
	t.Helper()
	fn, ok := scope.DefaultFunctions()[name]
	if !ok || fn == nil {
		t.Fatalf("DefaultFunctions()[%q] missing", name)
	}
	return fn
}

func TestPart6Evaluate(t *testing.T) {
	t.Run("arithmetic", func(t *testing.T) {
		if got, err := scope.Evaluate("1 + 2 * 3", nil, nil); err != nil || got != 7 {
			t.Errorf("Evaluate(1 + 2 * 3) = %v, %v; want 7, nil", got, err)
		}
		cases := []struct {
			expr string
			want float64
		}{
			{"(1 + 2) * 3", 9},
			{"10 / 4", 2.5},
			{"-7 % 3", -1},
			{"7 % -3", 1},
			{"+A1", 4},
			{"-A1", -4},
			{"A1 + B2*2", 9},
			{"A1 / 8 - B2", -2},
			{"(A1 - 1) * (B2 + 0.5)", 9},
		}
		for _, tc := range cases {
			t.Run(tc.expr, func(t *testing.T) {
				got, err := scope.Evaluate(tc.expr, formulaCells, nil)
				if err != nil {
					t.Fatalf("Evaluate(%q): unexpected error: %v", tc.expr, err)
				}
				if got != tc.want {
					t.Errorf("Evaluate(%q) = %v; want %v", tc.expr, got, tc.want)
				}
			})
		}
	})

	t.Run("literal forms", func(t *testing.T) {
		cases := []struct {
			expr string
			want float64
		}{
			{"1_000 + 0x10 + 0b1 + 0o7 + 010", 1032},
			{"1.5e3", 1500},
			{"0x1p4", 16},
			{"2.5", 2.5},
			{".5 + 1.", 1.5},
		}
		for _, tc := range cases {
			t.Run(tc.expr, func(t *testing.T) {
				got, err := scope.Evaluate(tc.expr, nil, nil)
				if err != nil {
					t.Fatalf("Evaluate(%q): unexpected error: %v", tc.expr, err)
				}
				if got != tc.want {
					t.Errorf("Evaluate(%q) = %v; want %v", tc.expr, got, tc.want)
				}
			})
		}
	})

	t.Run("built-in functions", func(t *testing.T) {
		if got, err := scope.Evaluate("ABS(-3)", nil, nil); err != nil || got != 3 {
			t.Errorf("Evaluate(ABS(-3)) = %v, %v; want 3, nil", got, err)
		}
		cases := []struct {
			expr string
			want float64
		}{
			{"MIN(3, 1, 2)", 1},
			{"MAX(3, 1, A1)", 4},
			{"ROUND(2.5)", 3},
			{"ROUND(-2.5)", -3},
			{"ROUND(A1 / 3)", 1},
			{"MAX(A1, B2) * 2", 8},
			{"ABS(MIN(-5, 2))", 5},
		}
		for _, tc := range cases {
			t.Run(tc.expr, func(t *testing.T) {
				got, err := scope.Evaluate(tc.expr, formulaCells, nil)
				if err != nil {
					t.Fatalf("Evaluate(%q): unexpected error: %v", tc.expr, err)
				}
				if got != tc.want {
					t.Errorf("Evaluate(%q) = %v; want %v", tc.expr, got, tc.want)
				}
			})
		}
	})

	t.Run("caller functions and overrides", func(t *testing.T) {
		funcs := map[string]scope.Func{
			"DOUBLE": func(args ...float64) (float64, error) { return args[0] * 2, nil },
			"ABS":    func(args ...float64) (float64, error) { return 42, nil },
		}
		if got, err := scope.Evaluate("DOUBLE(A1)", formulaCells, funcs); err != nil || got != 8 {
			t.Fatalf("Evaluate(DOUBLE(A1)) = %v, %v; want 8, nil", got, err)
		}
		if got, err := scope.Evaluate("ABS(1)", nil, funcs); err != nil || got != 42 {
			t.Errorf("Evaluate(ABS(1)) with override = %v, %v; want 42, nil", got, err)
		}
	})

	t.Run("nil maps are allowed", func(t *testing.T) {
		if got, err := scope.Evaluate("1+1", nil, nil); err != nil || got != 2 {
			t.Errorf("Evaluate(1+1, nil, nil) = %v, %v; want 2, nil", got, err)
		}
	})
}

func TestPart6Forbidden(t *testing.T) {
	t.Run("valid formula still evaluates", func(t *testing.T) {
		if got, err := scope.Evaluate("A1 + 1", formulaCells, nil); err != nil || got != 5 {
			t.Fatalf("Evaluate(A1 + 1) = %v, %v; want 5, nil", got, err)
		}
	})

	exprs := []string{
		"1 // 2",
		"1 /* 2 */ + 3",
		"A1 == 4",
		"A1 < 2",
		"A1 && B2",
		"true && false",
		"!A1",
		"^1",
		"&A1",
		"*A1",
		"A1 ** 2",
		"1 & 2",
		"1 | 2",
		"1 << 2",
		"A1 >> 1",
		"A1 &^ 1",
		"1 != 2",
		"A1.x",
		"A1[0]",
		`"s"`,
		"'c'",
		"2i",
		"func() float64 { return 1 }()",
		"[]float64{1}[0]",
		"A1.(int)",
		"MIN(A1...)",
		"(ABS)(1)",
		"(A1)(1)",
	}
	for _, expr := range exprs {
		t.Run(fmt.Sprintf("%q", expr), func(t *testing.T) {
			_, err := scope.Evaluate(expr, formulaCells, nil)
			if !errors.Is(err, scope.ErrForbidden) {
				t.Errorf("Evaluate(%q) error = %v; want ErrForbidden", expr, err)
			}
		})
	}
}

func TestPart6ParseErrors(t *testing.T) {
	t.Run("valid formula still evaluates", func(t *testing.T) {
		if got, err := scope.Evaluate("1", nil, nil); err != nil || got != 1 {
			t.Fatalf("Evaluate(1) = %v, %v; want 1, nil", got, err)
		}
	})

	exprs := []string{
		"",
		"   ",
		"1 +",
		"(1",
		"1 2",
		"A1 = 2",
		"A1 := 2",
		"99999999999999999999999",
	}
	for _, expr := range exprs {
		t.Run(fmt.Sprintf("%q", expr), func(t *testing.T) {
			_, err := scope.Evaluate(expr, formulaCells, nil)
			if !errors.Is(err, scope.ErrParse) {
				t.Errorf("Evaluate(%q) error = %v; want ErrParse", expr, err)
			}
		})
	}
}

func TestPart6Names(t *testing.T) {
	t.Run("known cell resolves", func(t *testing.T) {
		if got, err := scope.Evaluate("A1", formulaCells, nil); err != nil || got != 4 {
			t.Fatalf("Evaluate(A1) = %v, %v; want 4, nil", got, err)
		}
	})

	exprs := []string{
		"Z9 + 1",
		"ABS",
		"NOPE(1)",
		"true",
		"nil",
		"A1(1)",
		"MIN(Q1)",
	}
	for _, expr := range exprs {
		t.Run(expr, func(t *testing.T) {
			_, err := scope.Evaluate(expr, formulaCells, nil)
			if !errors.Is(err, scope.ErrUnknownName) {
				t.Errorf("Evaluate(%q) error = %v; want ErrUnknownName", expr, err)
			}
		})
	}
}

func TestPart6Errors(t *testing.T) {
	t.Run("half is exact", func(t *testing.T) {
		if got, err := scope.Evaluate("1/2", nil, nil); err != nil || got != 0.5 {
			t.Fatalf("Evaluate(1/2) = %v, %v; want 0.5, nil", got, err)
		}
	})

	exprs := []string{
		"1 / 0",
		"1 % 0",
		"0.0 / 0.0",
		"A1 / (B2 - 2.5)",
		"A1 % (B2 - 2.5)",
		"ABS(1, 2)",
		"ABS()",
		"MIN()",
		"ROUND(1, 2)",
		"1e308 * 10",
	}
	for _, expr := range exprs {
		t.Run(expr, func(t *testing.T) {
			_, err := scope.Evaluate(expr, formulaCells, nil)
			if !errors.Is(err, scope.ErrEval) {
				t.Errorf("Evaluate(%q) error = %v; want ErrEval", expr, err)
			}
		})
	}

	t.Run("function error wraps both sentinels", func(t *testing.T) {
		funcs := map[string]scope.Func{
			"FAIL": func(args ...float64) (float64, error) { return 0, errBoom },
		}
		if got, err := scope.Evaluate("1 + 1", nil, funcs); err != nil || got != 2 {
			t.Fatalf("Evaluate(1 + 1) = %v, %v; want 2, nil", got, err)
		}
		_, err := scope.Evaluate("FAIL(1)", nil, funcs)
		if !errors.Is(err, scope.ErrEval) {
			t.Errorf("error = %v; want ErrEval", err)
		}
		if !errors.Is(err, errBoom) {
			t.Errorf("error = %v; want the original function error wrapped", err)
		}
	})
}

func TestPart6Conflicts(t *testing.T) {
	t.Run("non-conflicting cell resolves", func(t *testing.T) {
		if got, err := scope.Evaluate("A1", map[string]float64{"A1": 1}, nil); err != nil || got != 1 {
			t.Fatalf("Evaluate(A1) = %v, %v; want 1, nil", got, err)
		}
	})

	t.Run("cell named like a default function", func(t *testing.T) {
		cells := map[string]float64{"A1": 1, "ABS": 2}
		_, err := scope.Evaluate("A1 + 1", cells, nil)
		if !errors.Is(err, scope.ErrConflict) {
			t.Errorf("error = %v; want ErrConflict", err)
		}
	})

	t.Run("cell named like a caller function", func(t *testing.T) {
		cells := map[string]float64{"A1": 1, "DOUBLE": 2}
		funcs := map[string]scope.Func{
			"DOUBLE": func(args ...float64) (float64, error) { return args[0] * 2, nil },
		}
		_, err := scope.Evaluate("A1 + 1", cells, funcs)
		if !errors.Is(err, scope.ErrConflict) {
			t.Errorf("error = %v; want ErrConflict", err)
		}
	})
}

func TestPart6FunctionsMerge(t *testing.T) {
	t.Run("DefaultFunctions has the four built-ins", func(t *testing.T) {
		got := slices.Sorted(maps.Keys(scope.DefaultFunctions()))
		if diff := cmp.Diff([]string{"ABS", "MAX", "MIN", "ROUND"}, got); diff != "" {
			t.Errorf("DefaultFunctions() keys mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("each call returns a fresh map", func(t *testing.T) {
		m1 := scope.DefaultFunctions()
		delete(m1, "ABS")
		m2 := scope.DefaultFunctions()
		if _, ok := m2["ABS"]; !ok {
			t.Error(`second DefaultFunctions() lost "ABS" after the first map was modified`)
		}
	})

	t.Run("caller funcs are not mutated", func(t *testing.T) {
		funcs := map[string]scope.Func{
			"X": func(args ...float64) (float64, error) { return 7, nil },
		}
		if got, err := scope.Evaluate("X(1)", nil, funcs); err != nil || got != 7 {
			t.Fatalf("Evaluate(X(1)) = %v, %v; want 7, nil", got, err)
		}
		if len(funcs) != 1 {
			t.Errorf("caller funcs has %d entries after Evaluate; want 1", len(funcs))
		}
	})

	t.Run("override applies to one call only", func(t *testing.T) {
		override := map[string]scope.Func{
			"ABS": func(args ...float64) (float64, error) { return 42, nil },
		}
		if got, err := scope.Evaluate("ABS(-1)", nil, override); err != nil || got != 42 {
			t.Fatalf("Evaluate(ABS(-1)) with override = %v, %v; want 42, nil", got, err)
		}
		if got, err := scope.Evaluate("ABS(-1)", nil, nil); err != nil || got != 1 {
			t.Errorf("Evaluate(ABS(-1)) after override = %v, %v; want 1, nil", got, err)
		}
	})
}

func TestPart6DefaultFunctions(t *testing.T) {
	t.Run("ABS and MIN/MAX", func(t *testing.T) {
		if got, err := mustDefault(t, "ABS")(-2); err != nil || got != 2 {
			t.Fatalf("ABS(-2) = %v, %v; want 2, nil", got, err)
		}
		if got, err := mustDefault(t, "MIN")(4, 2, 8); err != nil || got != 2 {
			t.Errorf("MIN(4, 2, 8) = %v, %v; want 2, nil", got, err)
		}
		if got, err := mustDefault(t, "MAX")(4, 2, 8); err != nil || got != 8 {
			t.Errorf("MAX(4, 2, 8) = %v, %v; want 8, nil", got, err)
		}
	})

	t.Run("ROUND half away from zero", func(t *testing.T) {
		round := mustDefault(t, "ROUND")
		cases := []struct {
			in, want float64
		}{
			{0.5, 1},
			{-0.5, -1},
			{1.4999, 1},
			{-1.5, -2},
		}
		for _, tc := range cases {
			if got, err := round(tc.in); err != nil || got != tc.want {
				t.Errorf("ROUND(%v) = %v, %v; want %v, nil", tc.in, got, err, tc.want)
			}
		}
	})

	t.Run("arity errors", func(t *testing.T) {
		if _, err := mustDefault(t, "ABS")(1, 2); err == nil {
			t.Error("ABS(1, 2): want error")
		}
		if _, err := mustDefault(t, "MIN")(); err == nil {
			t.Error("MIN(): want error")
		}
		if _, err := mustDefault(t, "ROUND")(); err == nil {
			t.Error("ROUND(): want error")
		}
	})
}
