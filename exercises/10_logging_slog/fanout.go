package shoplog

import "log/slog"

// NewFanout returns a handler that delivers each record to every child that is
// enabled for its level. Each child receives its own copy of the record.
// Handle reports the joined errors of all children and never stops early.
// Zero children means nothing is enabled. See README, Part 4.
func NewFanout(hs ...slog.Handler) slog.Handler {
	return nil
}
