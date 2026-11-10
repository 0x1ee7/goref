package scope

import (
	"errors"
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
	return ""
}

// ParseWindow parses "HH:MM-HH:MM". The rules are:
//   - exactly one '-' separates the two times;
//   - each time is 24-hour HH:MM with two digits for hour (00-23) and minute (00-59);
//   - the end must be strictly after the start.
//
// Every violation returns an error that wraps ErrInvalidWindow and includes the input.
func ParseWindow(s string) (Window, error) {
	return Window{}, errors.New("not implemented")
}
