package shoplog

import "log/slog"

// NewComponentLevels gates records per component. The threshold of a record
// is the level of its component (the top-level "component" attribute, from
// WithAttrs or from the record itself), or def when the component is absent or
// unknown. Enabled is a conservative prefilter and Handle is exact. next's own
// Enabled still applies, so give next the lowest level you want to allow.
// The levels map is copied. See README, Part 5.
func NewComponentLevels(next slog.Handler, def slog.Level, levels map[string]slog.Level) slog.Handler {
	return nil
}
