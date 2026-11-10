// Package perf covers performance engineering in Go: zero-allocation parsing,
// exact-size building, preallocation, sync.Pool discipline, struct layout and
// data layout. Allocation budgets are enforced with testing.AllocsPerRun, which
// counts allocations deterministically (unlike timings). Benchmarks in
// bench_test.go are for manual measurement with -benchmem.
//
// The package is named perf; the directory is 20_performance.
package perf

import "errors"

// Sentinel errors returned by ParseLine.
var (
	// ErrSyntax reports a token that is not key=value (no '=', or an empty key).
	ErrSyntax = errors.New("perf: syntax error")
	// ErrMissingField reports that a required key (level, route, status, dur_ms) is absent.
	ErrMissingField = errors.New("perf: missing field")
	// ErrBadValue reports a required value that is empty, or a status/dur_ms
	// that is not a non-negative decimal integer fitting in int.
	ErrBadValue = errors.New("perf: bad value")
)

// Line is one parsed access-log record. Level and Route are views into the
// buffer passed to ParseLine, not copies: they are valid only while that buffer
// is unchanged, and they keep it reachable.
type Line struct {
	Level  []byte
	Route  []byte
	Status int
	DurMS  int
}

// ParseLine parses one logfmt-style line such as
//
//	level=info route=/api/orders status=200 dur_ms=12
//
// The full grammar is in README.md, Part 1. A successful parse must not allocate.
func ParseLine(line []byte) (Line, error) {
	return Line{}, nil
}
