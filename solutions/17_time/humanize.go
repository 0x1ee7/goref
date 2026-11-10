// Part 3 (continued): humanized relative time.
package timeutil

import (
	"fmt"
	"time"
)

// Humanize describes when relative to now, using absolute time. Rules, by the
// absolute gap between when and now:
//   - under 45 seconds: "just now"
//   - under 1 hour: minutes, at least 1: "1 minute ago", "in 59 minutes"
//   - under 24 hours: hours: "3 hours ago", "in 1 hour"
//   - under 30 days: days: "2 days ago", "in 2 days"
//   - otherwise: "on YYYY-MM-DD", the date of when in when's own zone
//
// Future instants read "in ...", past instants read "... ago".
func Humanize(when, now time.Time) string {
	past := when.Before(now)
	gap := when.Sub(now)
	if gap < 0 {
		gap = -gap
	}

	switch {
	case gap < 45*time.Second:
		return "just now"
	case gap < time.Hour:
		return relative(int(max(1, gap/time.Minute)), "minute", past)
	case gap < oneDay:
		return relative(int(gap/time.Hour), "hour", past)
	case gap < 30*oneDay:
		return relative(int(gap/oneDay), "day", past)
	default:
		return "on " + when.Format(time.DateOnly)
	}
}

// relative formats a count and unit with a direction, pluralizing unless n is 1.
func relative(n int, unit string, past bool) string {
	if n != 1 {
		unit += "s"
	}
	if past {
		return fmt.Sprintf("%d %s ago", n, unit)
	}
	return fmt.Sprintf("in %d %s", n, unit)
}
