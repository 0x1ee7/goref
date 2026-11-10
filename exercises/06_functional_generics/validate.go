package functional

import "errors"

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
	return func(value any) []error { return nil }
}

// IsList accepts []any. Each only checks elements of []any, so pair it with
// IsList when a value must be a list.
func IsList() Rule {
	return func(value any) []error { return nil }
}

// MinLength checks strings only, counting runes so that multi-byte characters
// count once.
func MinLength(n int) Rule {
	return func(value any) []error { return nil }
}

// FullMatch checks that a string matches pattern in its entirety, like Python's
// re.fullmatch. Go's regexp has no fullmatch, so the pattern is wrapped in
// ^(?:…)$. Go's $ matches only at the very end of the text, unlike Python's $,
// which also matches before a trailing newline. It panics on an invalid pattern,
// like regexp.MustCompile, because that is a programmer error.
func FullMatch(pattern string) Rule {
	return func(value any) []error { return nil }
}

// InRange checks an integer or float value against the inclusive range [lo, hi].
// Bools and other types are outside its domain and pass.
func InRange(lo, hi float64) Rule {
	return func(value any) []error { return nil }
}

// AllOf runs every rule and concatenates their errors in order. It never
// short-circuits, so a form can report every problem at once.
func AllOf(rules ...Rule) Rule {
	return func(value any) []error { return nil }
}

// AnyOf passes on the first rule that passes and does not run the rest. If all
// fail, it returns every error. It panics when no rules are given: an empty
// disjunction is always false, which is a programmer error.
func AnyOf(rules ...Rule) Rule {
	return func(value any) []error { return nil }
}

// Field applies rule to record[name] and prefixes each error with "name: ". A
// missing key yields "name: missing". A value that is not an object yields no
// errors here; the object check belongs to the caller.
func Field(name string, rule Rule) Rule {
	return func(value any) []error { return nil }
}

// OptionalField is Field for keys that may be absent. A missing key is valid.
func OptionalField(name string, rule Rule) Rule {
	return func(value any) []error { return nil }
}

// Optional treats null as valid and delegates any other value.
func Optional(rule Rule) Rule {
	return func(value any) []error { return nil }
}

// Each applies rule to every element of a []any and prefixes errors with "[i] ".
func Each(rule Rule) Rule {
	return func(value any) []error { return nil }
}

// Validate runs rule and joins its errors into one error, or returns nil when
// the value is valid. errors.Join keeps each error inspectable.
func Validate(rule Rule, value any) error {
	return nil
}

// ValidateSignup checks a signup payload. A non-object is reported first and the
// field rules are skipped. Errors come out in field order: username, age, email,
// then the optional tags.
func ValidateSignup(payload any) []error {
	return nil
}
