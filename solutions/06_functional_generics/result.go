package functional

import (
	"errors"
	"fmt"
	"iter"
	"slices"
	"strconv"
	"strings"
)

// Result carries either a value or an error. Go's (T, error) is the idiomatic
// form; Result exists for pipelines where short-circuiting through MapResult,
// BindResult or CollectResults reads better than repeated err checks. Fields are
// unexported so that a Result can only be built by Ok or Err.
//
// The zero Result is Ok holding the zero T.
type Result[T any] struct {
	value T
	err   error
}

// Ok wraps a successful value.
func Ok[T any](v T) Result[T] { return Result[T]{value: v} }

// Err wraps a failure. Err(nil) is indistinguishable from Ok(zero), so never pass
// nil.
func Err[T any](err error) Result[T] { return Result[T]{err: err} }

// Unwrap returns the pair that idiomatic Go code would have returned directly.
func (r Result[T]) Unwrap() (T, error) { return r.value, r.err }

// IsOK reports whether r holds a value.
func (r Result[T]) IsOK() bool { return r.err == nil }

// OrElse returns the value, or fallback when r holds an error.
func (r Result[T]) OrElse(fallback T) T {
	if r.err != nil {
		return fallback
	}
	return r.value
}

// MapResult transforms the value of r. Methods cannot take their own type
// parameters, so these are package functions.
func MapResult[T, U any](r Result[T], f func(T) U) Result[U] {
	if r.err != nil {
		return Err[U](r.err)
	}
	return Ok(f(r.value))
}

// BindResult chains a step that can itself fail. f is not called after an error.
func BindResult[T, U any](r Result[T], f func(T) Result[U]) Result[U] {
	if r.err != nil {
		return Err[U](r.err)
	}
	return f(r.value)
}

// Lift adapts an idiomatic (U, error) function to return a Result.
func Lift[T, U any](f func(T) (U, error)) func(T) Result[U] {
	return func(t T) Result[U] {
		u, err := f(t)
		if err != nil {
			return Err[U](err)
		}
		return Ok(u)
	}
}

// CollectResults turns a sequence of Results into the values, or returns the
// first error. It stops consuming rs at the first failure, so later steps never
// run. The returned slice is never nil on success.
func CollectResults[T any](rs iter.Seq[Result[T]]) ([]T, error) {
	out := []T{}
	for r := range rs {
		v, err := r.Unwrap()
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// ParsePort parses a TCP port in 1..65535. Surrounding whitespace is ignored.
func ParsePort(raw string) (int, error) {
	s := strings.TrimSpace(raw)
	port, err := strconv.Atoi(s)
	if errors.Is(err, strconv.ErrRange) {
		return 0, errors.New("port must be between 1 and 65535")
	}
	if err != nil {
		return 0, fmt.Errorf("port must be an integer, got %q", s)
	}
	if port < 1 || port > 65535 {
		return 0, errors.New("port must be between 1 and 65535")
	}
	return port, nil
}

// ParsePorts parses a comma-separated list of ports. It fails on the first bad
// entry, and later entries are never parsed.
func ParsePorts(raw string) ([]int, error) {
	return CollectResults(MapSeq(slices.Values(strings.Split(raw, ",")), Lift(ParsePort)))
}

// ParseBool parses a case-insensitive boolean: true/yes/1/on or false/no/0/off.
func ParseBool(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "yes", "1", "on":
		return true, nil
	case "false", "no", "0", "off":
		return false, nil
	}
	return false, fmt.Errorf("expected boolean, got '%s'", raw)
}
