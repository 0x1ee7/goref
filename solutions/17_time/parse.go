// Part 2: parsing heterogeneous timestamps into UTC.
package timeutil

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseTimestamp parses a timestamp from a log line, client request, or payment
// provider and returns it in UTC. Accepted forms:
//   - RFC 3339 / ISO 8601 with a mandatory offset or "Z". Fractional seconds are allowed.
//   - RFC 1123Z (RFC 2822), e.g. "Sun, 29 Mar 2026 12:00:00 +0200". The weekday must match the date.
//   - A decimal integer Unix epoch, interpreted by [FromEpoch].
//
// Surrounding whitespace is trimmed. Naive, date-only, and malformed input
// return [ErrInvalidTimestamp]. An offset of "-0000" or "-00:00" means the
// offset is unknown and returns [ErrUnknownOffset].
func ParseTimestamp(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("%w: empty input", ErrInvalidTimestamp)
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return FromEpoch(n), nil
	}

	// Go's parser reads "-00:00" and "-0000" as an ordinary zero offset, so the
	// unknown-offset marker has to be detected on the input text.
	unknownOffset := strings.HasSuffix(s, "-0000") || strings.HasSuffix(s, "-00:00")

	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		if unknownOffset {
			return time.Time{}, fmt.Errorf("%w: %q", ErrUnknownOffset, s)
		}
		return t.UTC(), nil
	}

	if t, err := time.Parse(time.RFC1123Z, s); err == nil {
		if unknownOffset {
			return time.Time{}, fmt.Errorf("%w: %q", ErrUnknownOffset, s)
		}
		// time.Parse does not check the weekday name against the date.
		if !strings.HasPrefix(s, t.Weekday().String()[:3]) {
			return time.Time{}, fmt.Errorf("%w: weekday does not match date in %q", ErrInvalidTimestamp, s)
		}
		return t.UTC(), nil
	}

	return time.Time{}, fmt.Errorf("%w: %q", ErrInvalidTimestamp, s)
}

// epochMilliThreshold separates epoch seconds from epoch milliseconds. Seconds
// reach 1e11 in the year 5138, and milliseconds reach it in 1973.
const epochMilliThreshold = 100_000_000_000

// FromEpoch converts a Unix epoch value to UTC. A magnitude of at least 1e11
// means milliseconds; otherwise the value is seconds.
func FromEpoch(n int64) time.Time {
	if n >= epochMilliThreshold || n <= -epochMilliThreshold {
		return time.UnixMilli(n).UTC()
	}
	return time.Unix(n, 0).UTC()
}

// EpochMilli returns the whole milliseconds since 1970-01-01T00:00:00Z,
// rounded toward the past, so pre-epoch sub-millisecond instants round down.
func EpochMilli(t time.Time) int64 {
	// UnixMilli floors because the nanosecond field is always in [0, 1e9).
	// Duration.Milliseconds, applied to a Duration since the epoch, truncates
	// toward zero and would round pre-epoch instants up.
	return t.UnixMilli()
}
