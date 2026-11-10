package typed

import (
	"cmp"
	"errors"
)

// ErrEmpty reports an operation that needs at least one element.
var ErrEmpty = errors.New("empty input")

// ErrBounds reports lo > hi in Clamp.
var ErrBounds = errors.New("lower bound above upper bound")

// Number is a type set of numeric kinds. The ~ admits defined types such as
// `type Cents int64`, not just the predeclared types.
type Number interface {
	~int | ~int32 | ~int64 | ~float64
}

// Sum returns the sum of xs; the sum of no values is the zero value.
func Sum[T Number](xs []T) T {
	var zero T
	return zero
}

// Max returns the largest element of xs, or ErrEmpty.
func Max[T cmp.Ordered](xs []T) (T, error) {
	var zero T
	return zero, nil
}

// Clamp returns v limited to [lo, hi]. It returns ErrBounds when lo > hi.
func Clamp[T cmp.Ordered](v, lo, hi T) (T, error) {
	var zero T
	return zero, nil
}

// GroupBy partitions xs by key, preserving input order inside each group.
// The result is never nil; empty input yields an empty map.
func GroupBy[K comparable, V any](xs []V, key func(V) K) map[K][]V {
	return nil
}

// SortedKeys returns the keys of m in ascending order. A nil or empty map
// yields a non-nil empty slice.
func SortedKeys[K cmp.Ordered, V any](m map[K]V) []K {
	return nil
}

// Port is a TCP/UDP port number in 1..65535.
type Port uint16

// ErrInvalidPort reports a malformed or out-of-range port.
var ErrInvalidPort = errors.New("invalid port")

// ParsePort parses a decimal port number. Surrounding whitespace is trimmed.
// Errors wrap ErrInvalidPort.
func ParsePort(s string) (Port, error) { return 0, nil }

// ErrMissing reports a required setting that is absent or blank.
var ErrMissing = errors.New("missing setting")

// Setting is a typed configuration value read from a string source such as
// an environment map. The zero value is not meaningful; build one with
// Required or WithDefault.
type Setting[T any] struct{}

// Required returns a setting that must be present and non-blank.
func Required[T any](key string, parse func(string) (T, error)) Setting[T] {
	return Setting[T]{}
}

// WithDefault returns a setting that falls back to def when it is absent or
// blank. A present, non-blank value is never replaced by the default, even if
// it fails to parse.
func WithDefault[T any](key string, def T, parse func(string) (T, error)) Setting[T] {
	return Setting[T]{}
}

// Key returns the environment key of the setting.
func (s Setting[T]) Key() string { return "" }

// Load reads the setting from env. The raw value is whitespace-trimmed before
// parse runs. A missing or blank required value yields ErrMissing. Parse
// failures are returned wrapped with the key and keep the parser's error
// chain (errors.Is works on it).
func (s Setting[T]) Load(env map[string]string) (T, error) {
	var zero T
	return zero, nil
}
