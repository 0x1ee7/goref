package scope

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrInvalidWindow is wrapped by every ParseWindow failure.
var ErrInvalidWindow = errors.New("invalid window")

// Window is a daily time range. Start and End are offsets from midnight.
type Window struct {
	Start time.Duration
	End   time.Duration
}

// String formats the window as "HH:MM-HH:MM", the input format of ParseWindow.
func (w Window) String() string {
	return formatClock(w.Start) + "-" + formatClock(w.End)
}

// ParseWindow parses "HH:MM-HH:MM". The rules are:
//   - exactly one '-' separates the two times;
//   - each time is 24-hour HH:MM with two digits for hour (00-23) and minute (00-59);
//   - the end must be strictly after the start.
//
// Every violation returns an error that wraps ErrInvalidWindow and includes the input.
func ParseWindow(s string) (Window, error) {
	start, end, ok := strings.Cut(s, "-")
	if !ok || strings.Contains(end, "-") {
		return Window{}, fmt.Errorf("%w: %q: want HH:MM-HH:MM", ErrInvalidWindow, s)
	}
	ws, err := parseClock(start)
	if err != nil {
		return Window{}, fmt.Errorf("%w: %q: %w", ErrInvalidWindow, s, err)
	}
	we, err := parseClock(end)
	if err != nil {
		return Window{}, fmt.Errorf("%w: %q: %w", ErrInvalidWindow, s, err)
	}
	if we <= ws {
		return Window{}, fmt.Errorf("%w: %q: end must be after start", ErrInvalidWindow, s)
	}
	return Window{Start: ws, End: we}, nil
}

// parseClock parses a strict HH:MM into an offset from midnight.
func parseClock(s string) (time.Duration, error) {
	if len(s) != 5 || s[2] != ':' {
		return 0, fmt.Errorf("clock %q is not HH:MM", s)
	}
	h, ok1 := twoDigits(s[0], s[1])
	m, ok2 := twoDigits(s[3], s[4])
	if !ok1 || !ok2 || h > 23 || m > 59 {
		return 0, fmt.Errorf("clock %q out of range", s)
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute, nil
}

func twoDigits(a, b byte) (int, bool) {
	if a < '0' || a > '9' || b < '0' || b > '9' {
		return 0, false
	}
	return int(a-'0')*10 + int(b-'0'), true
}

func formatClock(d time.Duration) string {
	h := int(d / time.Hour)
	m := int(d % time.Hour / time.Minute)
	return fmt.Sprintf("%02d:%02d", h, m)
}
