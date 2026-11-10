package shoplog

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// ErrInvalidSampling is returned (wrapped) by NewSampler for bad arguments.
var ErrInvalidSampling = fmt.Errorf("shoplog: invalid sampling parameters")

// samplerState holds per-message occurrence counts, shared by derived handlers.
type samplerState struct {
	mu     sync.Mutex
	counts map[string]int
}

// sampler keeps the first `first` occurrences of each message and then every
// `thereafter`-th one. WARN and above always pass.
type sampler struct {
	next       slog.Handler
	first      int
	thereafter int
	st         *samplerState
}

// NewSampler returns a handler that rate-limits repeated messages below WARN.
// Occurrence n of a message is kept when n <= first or (n-first) is a multiple
// of thereafter. Counters are per message text and shared by all handlers
// derived from the returned one. first must be >= 0 and thereafter >= 1.
// See README, Part 5.
func NewSampler(next slog.Handler, first, thereafter int) (slog.Handler, error) {
	if first < 0 || thereafter < 1 {
		return nil, fmt.Errorf("%w: first=%d thereafter=%d", ErrInvalidSampling, first, thereafter)
	}
	return &sampler{
		next:       next,
		first:      first,
		thereafter: thereafter,
		st:         &samplerState{counts: make(map[string]int)},
	}, nil
}

// Enabled delegates to next; counting happens in Handle, so Enabled stays pure.
func (s *sampler) Enabled(ctx context.Context, lv slog.Level) bool {
	return s.next.Enabled(ctx, lv)
}

// Handle counts the occurrence and forwards it when the sampling rule keeps it.
func (s *sampler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelWarn {
		return s.next.Handle(ctx, r)
	}
	s.st.mu.Lock()
	s.st.counts[r.Message]++
	n := s.st.counts[r.Message]
	s.st.mu.Unlock()
	if n <= s.first || (n-s.first)%s.thereafter == 0 {
		return s.next.Handle(ctx, r)
	}
	return nil
}

// WithAttrs returns a sampler over next.WithAttrs sharing the same counters.
func (s *sampler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &sampler{next: s.next.WithAttrs(attrs), first: s.first, thereafter: s.thereafter, st: s.st}
}

// WithGroup returns a sampler over next.WithGroup sharing the same counters.
func (s *sampler) WithGroup(name string) slog.Handler {
	return &sampler{next: s.next.WithGroup(name), first: s.first, thereafter: s.thereafter, st: s.st}
}
