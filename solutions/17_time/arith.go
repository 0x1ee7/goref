// Part 3: calendar versus elapsed arithmetic, and Duration overflow.
package timeutil

import (
	"fmt"
	"math"
	"time"
)

// AddCalendarDays moves t by whole calendar days. The wall-clock time and zone
// are kept, and the result is resolved with GapShiftForward and OverlapEarlier
// (see [Localize]). This differs from t.Add(24*time.Hour), which moves by
// elapsed time.
func AddCalendarDays(t time.Time, days int) time.Time {
	d := civilUTC(t).AddDate(0, 0, days)
	wall := time.Date(d.Year(), d.Month(), d.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
	// GapShiftForward and OverlapEarlier never fail, so the error is always nil.
	res, _ := resolve(wall, t.Location(), GapShiftForward, OverlapEarlier)
	return res
}

// maxSpanSeconds is the largest whole-second span that fits in a Duration,
// with one second of margin for the sub-second part.
const maxSpanSeconds = math.MaxInt64/int64(time.Second) - 1

// ElapsedBetween returns end - start. It returns [ErrOverflow] when the
// difference is more than about 292 years, where time.Time.Sub would silently
// saturate.
func ElapsedBetween(start, end time.Time) (time.Duration, error) {
	span := end.Unix() - start.Unix()
	if span > maxSpanSeconds || span < -maxSpanSeconds {
		return 0, fmt.Errorf("%w: %v to %v", ErrOverflow, start, end)
	}
	return end.Sub(start), nil
}

// SafeAdd returns a + b, or [ErrOverflow] if the sum does not fit in a Duration.
// Duration addition wraps without any error.
func SafeAdd(a, b time.Duration) (time.Duration, error) {
	const maxD = time.Duration(math.MaxInt64)
	const minD = time.Duration(math.MinInt64)
	if (b > 0 && a > maxD-b) || (b < 0 && a < minD-b) {
		return 0, fmt.Errorf("%w: %v + %v", ErrOverflow, a, b)
	}
	return a + b, nil
}
