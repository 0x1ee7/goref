package shoplog

import (
	"io"
	"log/slog"
)

// NewLogfmtHandler returns a handler written from scratch against the
// slog.Handler contract. Each record is one line:
//
//	level=INFO msg="hello world" svc=api http.method=GET
//
// No timestamp is written. Groups flatten to dotted keys, values are
// resolved (LogValuer included) and quoted when they contain space, '=', '"',
// control characters or are empty. A nil level means INFO. See README, Part 3.
func NewLogfmtHandler(w io.Writer, level slog.Leveler) slog.Handler {
	return nil
}
