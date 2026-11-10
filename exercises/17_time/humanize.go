// Part 3 (continued): humanized relative time.
package timeutil

import "time"

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
	return ""
}
