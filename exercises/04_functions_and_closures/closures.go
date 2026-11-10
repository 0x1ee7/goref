// Package funcs is a refresher on Go functions: closures, defer and panic
// semantics, middleware chains, wrappers, function types as interfaces and
// generic higher-order helpers. README.md is the spec.
package funcs

// NewIDGenerator returns a generator whose IDs are "<prefix>-1", "<prefix>-2", ...
// Each call creates independent state. The returned function is safe for
// concurrent use and never returns the same ID twice.
func NewIDGenerator(prefix string) func() string {
	return func() string { return "" }
}

// Compose returns a function that applies steps left to right. The first
// failing step stops the chain and its error is returned wrapped as
// "step N: %w" (N is 1-based). With no steps the result is the identity.
func Compose[T any](steps ...func(T) (T, error)) func(T) (T, error) {
	return func(v T) (T, error) {
		var zero T
		return zero, nil
	}
}
