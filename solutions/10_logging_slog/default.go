package shoplog

import (
	"log/slog"
	"sync"
)

// UseDefault installs l as slog's default logger and returns a function that
// restores the logger that was current before the call. The restore function
// is idempotent: only its first call acts, so a stale call cannot undo a newer
// installation. See README, Part 7.
func UseDefault(l *slog.Logger) func() {
	prev := slog.Default()
	slog.SetDefault(l)
	var once sync.Once
	return func() {
		once.Do(func() { slog.SetDefault(prev) })
	}
}
