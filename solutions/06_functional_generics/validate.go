package functional

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"unicode/utf8"
)

// Rule judges one value. Input comes from untrusted, JSON-shaped data, so the
// value is `any`: objects are map[string]any, arrays are []any. A nil result
// means valid. Rules never panic on bad input.
type Rule func(value any) []error

// ErrMissing marks an absent record field. Field wraps it, so callers can test
// errors.Is(err, ErrMissing).
var ErrMissing = errors.New("missing")

// IsType accepts values of type T. Go's type identity already rejects bool for
// int; the gotcha is numbers from JSON, which decode as float64.
func IsType[T any]() Rule {
	want := reflect.TypeFor[T]().String()
	return func(value any) []error {
		if _, ok := value.(T); ok {
			return nil
		}
		return []error{fmt.Errorf("expected %s, got %s", want, kind(value))}
	}
}

// IsList accepts []any. Each only checks elements of []any, so pair it with
// IsList when a value must be a list.
func IsList() Rule {
	return func(value any) []error {
		if _, ok := value.([]any); ok {
			return nil
		}
		return []error{fmt.Errorf("expected list, got %s", kind(value))}
	}
}

// MinLength checks strings only, counting runes so that multi-byte characters
// count once.
func MinLength(n int) Rule {
	return func(value any) []error {
		s, ok := value.(string)
		if !ok {
			return nil
		}
		if utf8.RuneCountInString(s) < n {
			return []error{fmt.Errorf("length must be at least %d", n)}
		}
		return nil
	}
}

// FullMatch checks that a string matches pattern in its entirety, like Python's
// re.fullmatch. Go's regexp has no fullmatch, so the pattern is wrapped in
// ^(?:…)$. Go's $ matches only at the very end of the text, unlike Python's $,
// which also matches before a trailing newline. It panics on an invalid pattern,
// like regexp.MustCompile, because that is a programmer error.
func FullMatch(pattern string) Rule {
	re := regexp.MustCompile(`^(?:` + pattern + `)$`)
	return func(value any) []error {
		s, ok := value.(string)
		if !ok {
			return nil
		}
		if !re.MatchString(s) {
			return []error{fmt.Errorf("must match pattern '%s'", pattern)}
		}
		return nil
	}
}

// InRange checks an integer or float value against the inclusive range [lo, hi].
// Bools and other types are outside its domain and pass.
func InRange(lo, hi float64) Rule {
	return func(value any) []error {
		var x float64
		switch v := value.(type) {
		case int:
			x = float64(v)
		case int64:
			x = float64(v)
		case float64:
			x = v
		default:
			return nil
		}
		if x < lo || x > hi {
			return []error{fmt.Errorf("must be between %s and %s",
				strconv.FormatFloat(lo, 'f', -1, 64), strconv.FormatFloat(hi, 'f', -1, 64))}
		}
		return nil
	}
}

// AllOf runs every rule and concatenates their errors in order. It never
// short-circuits, so a form can report every problem at once.
func AllOf(rules ...Rule) Rule {
	return func(value any) []error {
		var errs []error
		for _, r := range rules {
			errs = append(errs, r(value)...)
		}
		return errs
	}
}

// AnyOf passes on the first rule that passes and does not run the rest. If all
// fail, it returns every error. It panics when no rules are given: an empty
// disjunction is always false, which is a programmer error.
func AnyOf(rules ...Rule) Rule {
	if len(rules) == 0 {
		panic("functional: AnyOf needs at least one rule")
	}
	return func(value any) []error {
		var errs []error
		for _, r := range rules {
			rerrs := r(value)
			if len(rerrs) == 0 {
				return nil
			}
			errs = append(errs, rerrs...)
		}
		return errs
	}
}

// Field applies rule to record[name] and prefixes each error with "name: ". A
// missing key yields "name: missing". A value that is not an object yields no
// errors here; the object check belongs to the caller.
func Field(name string, rule Rule) Rule {
	return field(name, rule, false)
}

// OptionalField is Field for keys that may be absent. A missing key is valid.
func OptionalField(name string, rule Rule) Rule {
	return field(name, rule, true)
}

func field(name string, rule Rule, optional bool) Rule {
	return func(value any) []error {
		rec, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		v, ok := rec[name]
		if !ok {
			if optional {
				return nil
			}
			return []error{fmt.Errorf("%s: %w", name, ErrMissing)}
		}
		return prefixAll(name+": ", rule(v))
	}
}

// Optional treats null as valid and delegates any other value.
func Optional(rule Rule) Rule {
	return func(value any) []error {
		if value == nil {
			return nil
		}
		return rule(value)
	}
}

// Each applies rule to every element of a []any and prefixes errors with "[i] ".
func Each(rule Rule) Rule {
	return func(value any) []error {
		items, ok := value.([]any)
		if !ok {
			return nil
		}
		var errs []error
		for i, item := range items {
			errs = append(errs, prefixAll(fmt.Sprintf("[%d] ", i), rule(item))...)
		}
		return errs
	}
}

// Validate runs rule and joins its errors into one error, or returns nil when
// the value is valid. errors.Join keeps each error inspectable.
func Validate(rule Rule, value any) error {
	return errors.Join(rule(value)...)
}

// ValidateSignup checks a signup payload. A non-object is reported first and the
// field rules are skipped. Errors come out in field order: username, age, email,
// then the optional tags.
func ValidateSignup(payload any) []error {
	if _, ok := payload.(map[string]any); !ok {
		return []error{fmt.Errorf("expected Mapping, got %s", kind(payload))}
	}
	return AllOf(
		Field("username", AllOf(IsType[string](), MinLength(3), FullMatch(`[a-z0-9_]+`))),
		Field("age", AllOf(IsType[int](), InRange(13, 120))),
		Field("email", AllOf(IsType[string](), FullMatch(`[^@\s]+@[^@\s]+\.[^@\s]+`))),
		OptionalField("tags", Optional(AllOf(IsList(), Each(AllOf(IsType[string](), MinLength(1)))))),
	)(payload)
}

// prefixAll returns errs with prefix added to each message. It returns nil for no
// errors, so "valid" is always nil.
func prefixAll(prefix string, errs []error) []error {
	if len(errs) == 0 {
		return nil
	}
	out := make([]error, len(errs))
	for i, e := range errs {
		out[i] = fmt.Errorf("%s%w", prefix, e)
	}
	return out
}

// kind names a value the way the messages above describe it.
func kind(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case map[string]any:
		return "Mapping"
	case []any:
		return "list"
	}
	return fmt.Sprintf("%T", v)
}
