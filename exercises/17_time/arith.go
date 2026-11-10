// Part 3: calendar versus elapsed arithmetic, and Duration overflow.
package timeutil

import "time"

// AddCalendarDays moves t by whole calendar days. The wall-clock time and zone
// are kept, and the result is resolved with GapShiftForward and OverlapEarlier
// (see [Localize]). This differs from t.Add(24*time.Hour), which moves by
// elapsed time.
func AddCalendarDays(t time.Time, days int) time.Time {
	return time.Time{}
}

// ElapsedBetween returns end - start. It returns [ErrOverflow] when the
// difference is more than about 292 years, where time.Time.Sub would silently
// saturate.
func ElapsedBetween(start, end time.Time) (time.Duration, error) {
	return 0, errNotImplemented
}

// SafeAdd returns a + b, or [ErrOverflow] if the sum does not fit in a Duration.
// Duration addition wraps without any error.
func SafeAdd(a, b time.Duration) (time.Duration, error) {
	return 0, errNotImplemented
}
