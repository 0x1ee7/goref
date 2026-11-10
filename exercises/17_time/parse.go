// Part 2: parsing heterogeneous timestamps into UTC.
package timeutil

import "time"

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
	return time.Time{}, errNotImplemented
}

// FromEpoch converts a Unix epoch value to UTC. A magnitude of at least 1e11
// means milliseconds; otherwise the value is seconds.
func FromEpoch(n int64) time.Time {
	return time.Time{}
}

// EpochMilli returns the whole milliseconds since 1970-01-01T00:00:00Z,
// rounded toward the past, so pre-epoch sub-millisecond instants round down.
func EpochMilli(t time.Time) int64 {
	return 0
}
