package functional_test

import (
	"errors"
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	fp "goref/exercises/06_functional_generics"
)

type ruleCase struct {
	name  string
	rule  fp.Rule
	value any
	want  []string
	// bad is a value the rule must reject. It is checked only when want is
	// empty, so valid-input subtests cannot pass against a stub that accepts
	// everything.
	bad any
}

func msgs(errs []error) []string {
	var out []string
	for _, e := range errs {
		out = append(out, e.Error())
	}
	return out
}

func control(t *testing.T, rule fp.Rule, bad any) {
	t.Helper()
	if len(rule(bad)) == 0 {
		t.Errorf("control: rule accepted %#v, which it must reject", bad)
	}
}

func assertPanics(t *testing.T, what string, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s: expected a panic", what)
		}
	}()
	f()
}

func TestPart6Rules(t *testing.T) {
	tests := []ruleCase{
		{"IsType[string] accepts a string", fp.IsType[string](), "x", nil, 3},
		{"IsType[string] rejects an int", fp.IsType[string](), 3, []string{"expected string, got int"}, nil},
		{"IsType[int] rejects a JSON float64", fp.IsType[int](), 3.0, []string{"expected int, got float64"}, nil},
		{"IsList accepts a list", fp.IsList(), []any{"a"}, nil, "x"},
		{"IsList rejects a string", fp.IsList(), "x", []string{"expected list, got string"}, nil},
		{"MinLength counts runes not bytes", fp.MinLength(3), "héé", nil, "hé"},
		{"MinLength rejects a short string", fp.MinLength(3), "ab", []string{"length must be at least 3"}, nil},
		{"MinLength ignores non-strings", fp.MinLength(3), 42, nil, "ab"},
		{"FullMatch accepts a whole match", fp.FullMatch(`[a-z]+`), "abc", nil, "ab1"},
		{"FullMatch rejects a partial match", fp.FullMatch(`[a-z]+`), "abc1", []string{"must match pattern '[a-z]+'"}, nil},
		{"FullMatch rejects a trailing newline", fp.FullMatch(`[a-z]+`), "abc\n", []string{"must match pattern '[a-z]+'"}, nil},
		{"InRange accepts the lower bound", fp.InRange(13, 120), 13, nil, 12},
		{"InRange rejects below range", fp.InRange(13, 120), 12, []string{"must be between 13 and 120"}, nil},
		{"InRange accepts int64", fp.InRange(13, 120), int64(50), nil, int64(200)},
		{"InRange rejects int64 above range", fp.InRange(13, 120), int64(200), []string{"must be between 13 and 120"}, nil},
		{"InRange fractional bounds", fp.InRange(0.5, 1.5), 2.0, []string{"must be between 0.5 and 1.5"}, nil},
		{"AllOf reports every failure in order", fp.AllOf(fp.MinLength(5), fp.FullMatch(`\d+`)), "ab",
			[]string{"length must be at least 5", `must match pattern '\d+'`}, nil},
		{"AllOf passes when every rule passes", fp.AllOf(fp.MinLength(2), fp.MinLength(3)), "abc", nil, "a"},
		{"AnyOf passes when one rule passes", fp.AnyOf(fp.IsType[int](), fp.IsType[string]()), "x", nil, true},
		{"AnyOf reports every failure when none pass", fp.AnyOf(fp.IsType[int](), fp.IsType[string]()), true,
			[]string{"expected int, got bool", "expected string, got bool"}, nil},
		{"Field validates a present key", fp.Field("username", fp.IsType[string]()),
			map[string]any{"username": "ada"}, nil, map[string]any{"username": 3}},
		{"Field prefixes errors with the key", fp.Field("username", fp.IsType[string]()),
			map[string]any{"username": 3}, []string{"username: expected string, got int"}, nil},
		{"Field reports a missing key", fp.Field("username", fp.IsType[string]()),
			map[string]any{}, []string{"username: missing"}, nil},
		{"Field ignores non-objects", fp.Field("a", fp.IsType[int]()), "x", nil, map[string]any{"a": "x"}},
		{"OptionalField allows a missing key", fp.OptionalField("tags", fp.IsType[string]()),
			map[string]any{}, nil, map[string]any{"tags": 3}},
		{"OptionalField still checks a present key", fp.OptionalField("tags", fp.IsType[string]()),
			map[string]any{"tags": 3}, []string{"tags: expected string, got int"}, nil},
		{"Optional accepts null", fp.Optional(fp.IsType[string]()), nil, nil, 3},
		{"Optional checks non-null values", fp.Optional(fp.IsType[string]()), 3,
			[]string{"expected string, got int"}, nil},
		{"Each indexes errors", fp.Each(fp.AllOf(fp.IsType[string](), fp.MinLength(1))), []any{"ok", 7, ""},
			[]string{"[1] expected string, got int", "[2] length must be at least 1"}, nil},
		{"Each accepts all valid items", fp.Each(fp.IsType[string]()), []any{"a", "b"}, nil, []any{"a", 1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := msgs(tc.rule(tc.value))
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("rule errors mismatch (-want +got):\n%s", diff)
			}
			if len(tc.want) == 0 {
				control(t, tc.rule, tc.bad)
			}
		})
	}
}

func TestPart6Combinators(t *testing.T) {
	t.Run("AnyOf stops at the first pass", func(t *testing.T) {
		calls := 0
		counting := func(any) []error {
			calls++
			return []error{errors.New("no")}
		}
		rule := fp.AnyOf(fp.IsType[string](), counting)
		if errs := rule("x"); len(errs) != 0 {
			t.Errorf("AnyOf on a passing value returned %v, want nil", msgs(errs))
		}
		if calls != 0 {
			t.Errorf("second rule ran %d times after the first passed, want 0", calls)
		}
		if errs := rule(3); len(errs) != 2 {
			t.Errorf("AnyOf on a failing value returned %d errors, want 2", len(errs))
		}
		if calls != 1 {
			t.Errorf("second rule ran %d times after the first failed, want 1", calls)
		}
	})
	t.Run("AnyOf with no rules panics", func(t *testing.T) {
		assertPanics(t, "AnyOf()", func() { fp.AnyOf() })
	})
	t.Run("FullMatch with an invalid pattern panics", func(t *testing.T) {
		assertPanics(t, `FullMatch("(")`, func() { fp.FullMatch("(") })
	})
}

func TestPart6Validate(t *testing.T) {
	t.Run("valid value returns nil", func(t *testing.T) {
		if err := fp.Validate(fp.IsType[int](), 3); err != nil {
			t.Errorf("Validate(valid) = %v, want nil", err)
		}
		// Control: the same rule must reject a bad value, so a stub that
		// always returns nil cannot pass this subtest.
		if err := fp.Validate(fp.IsType[int](), "x"); err == nil {
			t.Errorf("Validate(invalid) = nil, want an error")
		}
	})
	t.Run("invalid value returns the error", func(t *testing.T) {
		err := fp.Validate(fp.IsType[int](), "x")
		if err == nil || err.Error() != "expected int, got string" {
			t.Errorf("Validate(invalid) = %v, want \"expected int, got string\"", err)
		}
	})
	t.Run("joins every error", func(t *testing.T) {
		err := fp.Validate(fp.AllOf(fp.MinLength(5), fp.FullMatch(`\d+`)), "ab")
		want := "length must be at least 5\nmust match pattern '\\d+'"
		if err == nil || err.Error() != want {
			t.Errorf("Validate err = %v, want %q", err, want)
		}
	})
	t.Run("missing field wraps ErrMissing", func(t *testing.T) {
		err := fp.Validate(fp.Field("name", fp.IsType[string]()), map[string]any{})
		if !errors.Is(err, fp.ErrMissing) {
			t.Errorf("Validate(missing) = %v, want an error wrapping ErrMissing", err)
		}
	})
}

func signup(over map[string]any) map[string]any {
	p := map[string]any{"username": "ada_1", "age": 36, "email": "ada@example.com"}
	maps.Copy(p, over)
	return p
}

func TestPart6ValidateSignup(t *testing.T) {
	tests := []struct {
		name    string
		payload any
		want    []string
	}{
		{"valid payload", signup(nil), nil},
		{"tags as a valid list", signup(map[string]any{"tags": []any{"admin"}}), nil},
		{"tags may be null", signup(map[string]any{"tags": nil}), nil},
		{"payload must be an object", []any{}, []string{"expected Mapping, got list"}},
		{"null payload", nil, []string{"expected Mapping, got null"}},
		{"missing username", map[string]any{"age": 30, "email": "a@b.co"}, []string{"username: missing"}},
		{"username too short and invalid", signup(map[string]any{"username": "A!"}), []string{
			"username: length must be at least 3",
			"username: must match pattern '[a-z0-9_]+'",
		}},
		{"age out of range", signup(map[string]any{"age": 12}), []string{"age: must be between 13 and 120"}},
		{"age must be an int, not a float", signup(map[string]any{"age": 30.0}), []string{"age: expected int, got float64"}},
		{"invalid email", signup(map[string]any{"email": "nope"}), []string{
			`email: must match pattern '[^@\s]+@[^@\s]+\.[^@\s]+'`,
		}},
		{"tag item error is indexed", signup(map[string]any{"tags": []any{"ok", ""}}), []string{
			"tags: [1] length must be at least 1",
		}},
		{"every missing field reported together", map[string]any{}, []string{
			"username: missing", "age: missing", "email: missing",
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := msgs(fp.ValidateSignup(tc.payload))
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("ValidateSignup mismatch (-want +got):\n%s", diff)
			}
			if len(tc.want) == 0 && len(fp.ValidateSignup(map[string]any{})) == 0 {
				t.Error("control: ValidateSignup accepted an empty object")
			}
		})
	}
}
