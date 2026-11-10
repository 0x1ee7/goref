// Package funcs is the reference solution for the functions and closures
// exercise: closures that hold state, generic function composition, resource
// cleanup, panic containment, HTTP middleware, retries, handler adapters,
// command registries, and a single-flight memo.
package funcs

import (
	"fmt"
	"strconv"
	"sync/atomic"
)

// NewIDGenerator returns a function that yields "prefix-1", "prefix-2", and so
// on. Each generator owns its counter; the returned function is safe for
// concurrent use.
func NewIDGenerator(prefix string) func() string {
	var n atomic.Uint64
	return func() string {
		return prefix + "-" + strconv.FormatUint(n.Add(1), 10)
	}
}

// Compose returns a function that applies steps left to right, feeding each
// result into the next step. The first failing step stops the chain; its error
// is wrapped with the 1-based step number and the zero value is returned. With
// no steps, the result is the identity function.
func Compose[T any](steps ...func(T) (T, error)) func(T) (T, error) {
	return func(v T) (T, error) {
		for i, step := range steps {
			next, err := step(v)
			if err != nil {
				var zero T
				return zero, fmt.Errorf("step %d: %w", i+1, err)
			}
			v = next
		}
		return v, nil
	}
}
