// Part 5: recurring meetings with an explicit DST policy.
package timeutil

import "time"

// RecurrenceRule describes a weekly meeting at a local wall-clock time in
// Location. The zero value is invalid because Location is nil.
type RecurrenceRule struct {
	// Start is the earliest date considered. Its wall-clock date, read in its own location, is used.
	Start time.Time
	// Weekday is the weekday of every occurrence.
	Weekday time.Weekday
	// Hour (0-23) and Minute (0-59) are the local wall-clock time.
	Hour, Minute int
	// Location is the zone the wall-clock time is read in.
	Location *time.Location
	// Gap decides what happens when the wall-clock time falls in a DST gap.
	Gap GapPolicy
	// Overlap decides what happens when the wall-clock time falls in a DST overlap.
	Overlap OverlapPolicy
}

// Occurrences returns the next count occurrences, in ascending order, on or
// after Start's date. Occurrences dropped by GapSkip or OverlapSkip do not
// count toward count. It returns [ErrInvalid] for a negative count, a nil
// Location, a Weekday outside Sunday..Saturday, an Hour or Minute out of range,
// or an unknown policy.
func (r RecurrenceRule) Occurrences(count int) ([]time.Time, error) {
	return nil, errNotImplemented
}
