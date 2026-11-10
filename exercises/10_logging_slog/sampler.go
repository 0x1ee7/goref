package shoplog

import (
	"errors"
	"log/slog"
)

// ErrInvalidSampling is returned (wrapped) by NewSampler for bad arguments.
var ErrInvalidSampling = errors.New("shoplog: invalid sampling")

// NewSampler wraps next so that, per distinct message, the first `first`
// records below WARN pass and afterwards only every `thereafter`-th one passes
// (occurrence n passes when n <= first or (n-first) % thereafter == 0).
// Records at WARN or above always pass. Counters are shared by every handler
// derived from the returned one. first must be >= 0 and thereafter >= 1.
// See README, Part 5.
func NewSampler(next slog.Handler, first, thereafter int) (slog.Handler, error) {
	return nil, errors.New("not implemented")
}
