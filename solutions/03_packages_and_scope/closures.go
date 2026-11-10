package scope

import (
	"fmt"
	"sync/atomic"
)

// NewCounter returns a closure that yields start, start+step, start+2*step, ...
// Each call returns the current value and then advances. Counters do not share
// state. A zero step returns ErrInvalidArgument.
func NewCounter(start, step int) (func() int, error) {
	if step == 0 {
		return nil, fmt.Errorf("%w: step must be non-zero", ErrInvalidArgument)
	}
	next := start
	return func() int {
		v := next
		next += step
		return v
	}, nil
}

// NewIDGenerator returns a closure that yields "<prefix>-<n>", where n starts
// at 1 and is zero-padded to width digits (never truncated when longer). An
// empty prefix or width < 1 returns ErrInvalidArgument. The closure is safe
// for concurrent use: no two calls return the same id.
func NewIDGenerator(prefix string, width int) (func() string, error) {
	if prefix == "" || width < 1 {
		return nil, fmt.Errorf("%w: prefix %q, width %d", ErrInvalidArgument, prefix, width)
	}
	var n atomic.Int64
	return func() string {
		return fmt.Sprintf("%s-%0*d", prefix, width, n.Add(1))
	}, nil
}

// LoopCallbacks returns one callback per label. Each callback returns
// "clicked <label>" for its own label.
func LoopCallbacks(labels []string) []func() string {
	var out []func() string
	for _, label := range labels {
		// Since Go 1.22 each iteration has its own label, so capturing it is safe.
		out = append(out, func() string { return "clicked " + label })
	}
	return out
}

// SharedCallbacks returns one callback per label, written so that all of them
// share one variable declared outside the loop. Calling any of them returns
// "clicked <last label>". This is the classic late-binding bug, kept on
// purpose so you can see it. Empty labels yields no callbacks.
func SharedCallbacks(labels []string) []func() string {
	var (
		out     []func() string
		clicked string
	)
	for _, label := range labels {
		clicked = label
		out = append(out, func() string { return "clicked " + clicked })
	}
	return out
}

// NewAdders returns one function per offset; adder(v) returns v + offset.
func NewAdders(offsets []int) []func(int) int {
	out := make([]func(int) int, 0, len(offsets))
	for _, off := range offsets {
		out = append(out, func(v int) int { return v + off })
	}
	return out
}
