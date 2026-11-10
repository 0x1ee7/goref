// Package perf covers performance engineering in Go: zero-allocation parsing,
// exact-size building, preallocation, sync.Pool discipline, struct layout and
// data layout. Allocation budgets are enforced with testing.AllocsPerRun, which
// counts allocations deterministically (unlike timings). Benchmarks in
// bench_test.go are for manual measurement with -benchmem.
//
// The package is named perf; the directory is 20_performance.
package perf

import (
	"bytes"
	"errors"
	"fmt"
	"math"
)

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

// Bits recording which required keys have been seen.
const (
	seenLevel uint8 = 1 << iota
	seenRoute
	seenStatus
	seenDur
	seenAll = seenLevel | seenRoute | seenStatus | seenDur
)

// ParseLine parses one logfmt-style line such as
//
//	level=info route=/api/orders status=200 dur_ms=12
//
// The full grammar is in README.md, Part 1. A successful parse must not allocate.
//
// It is a single left-to-right pass. Keys are compared with string(key) == "...",
// which the compiler lowers to a byte comparison without a copy. Numbers are
// parsed by hand because strconv needs a string, and converting the value to a
// string would allocate. Errors are the only place fmt is used, so the success
// path stays allocation-free.
func ParseLine(line []byte) (Line, error) {
	line = trimEOL(line)
	var (
		out  Line
		seen uint8
	)
	for {
		for len(line) > 0 && line[0] == ' ' {
			line = line[1:]
		}
		if len(line) == 0 {
			break
		}
		tok := line
		if i := bytes.IndexByte(line, ' '); i >= 0 {
			tok, line = line[:i], line[i:]
		} else {
			line = nil
		}

		eq := bytes.IndexByte(tok, '=')
		if eq <= 0 {
			return Line{}, fmt.Errorf("%w: %q", ErrSyntax, tok)
		}
		key, val := tok[:eq], tok[eq+1:]

		switch {
		case string(key) == "level":
			if len(val) == 0 {
				return Line{}, fmt.Errorf("%w: level is empty", ErrBadValue)
			}
			out.Level = val
			seen |= seenLevel
		case string(key) == "route":
			if len(val) == 0 {
				return Line{}, fmt.Errorf("%w: route is empty", ErrBadValue)
			}
			out.Route = val
			seen |= seenRoute
		case string(key) == "status":
			n, ok := parseNonNeg(val)
			if !ok {
				return Line{}, fmt.Errorf("%w: status %q", ErrBadValue, val)
			}
			out.Status = n
			seen |= seenStatus
		case string(key) == "dur_ms":
			n, ok := parseNonNeg(val)
			if !ok {
				return Line{}, fmt.Errorf("%w: dur_ms %q", ErrBadValue, val)
			}
			out.DurMS = n
			seen |= seenDur
		default:
			// Unknown keys are valid and ignored, so logs can carry extra fields.
		}
	}
	if seen != seenAll {
		return Line{}, ErrMissingField
	}
	return out, nil
}

// trimEOL removes one trailing "\n" and then one trailing "\r".
func trimEOL(b []byte) []byte {
	if n := len(b); n > 0 && b[n-1] == '\n' {
		b = b[:n-1]
	}
	if n := len(b); n > 0 && b[n-1] == '\r' {
		b = b[:n-1]
	}
	return b
}

// parseNonNeg parses a non-empty run of ASCII digits that fits in int. It
// rejects signs, underscores and empty input.
func parseNonNeg(b []byte) (int, bool) {
	if len(b) == 0 {
		return 0, false
	}
	n := 0
	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, false
		}
		d := int(c - '0')
		// n*10+d <= MaxInt  <=>  n <= (MaxInt-d)/10 for non-negative n.
		if n > (math.MaxInt-d)/10 {
			return 0, false
		}
		n = n*10 + d
	}
	return n, true
}
