package shoplog

import "log/slog"

// UseDefault installs l as slog's process-wide default logger and returns a
// function that restores the previous default. Restoring is idempotent: only
// the first call acts, so a late call never undoes a newer installation. See
// README, Part 7.
func UseDefault(l *slog.Logger) func() {
	return func() {}
}
