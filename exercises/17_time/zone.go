// Part 1: instants, zones, and DST wall-clock policy.
package timeutil

import (
	"time"

	_ "time/tzdata" // embeds the IANA database; Windows has no system zoneinfo
)

// GapPolicy chooses what [Localize] returns for a wall-clock time that a DST
// transition skipped (02:30 on the spring-forward day in Europe/Berlin).
type GapPolicy int

const (
	// GapShiftForward returns the instant the clock jumps to: 02:30 becomes 03:30 CEST.
	GapShiftForward GapPolicy = iota
	// GapSkip reports [ErrNonexistent].
	GapSkip
)

// OverlapPolicy chooses which occurrence [Localize] returns for a wall-clock
// time that a DST transition repeated (02:30 on the fall-back day).
type OverlapPolicy int

const (
	// OverlapEarlier returns the first occurrence (the pre-transition offset, CEST).
	OverlapEarlier OverlapPolicy = iota
	// OverlapLater returns the second occurrence (the post-transition offset, CET).
	OverlapLater
	// OverlapSkip reports [ErrAmbiguous].
	OverlapSkip
)

// LoadZone loads an IANA zone by name, such as "Europe/Berlin". The names ""
// and "Local" are rejected because they depend on the machine. Errors wrap
// [ErrUnknownZone].
func LoadZone(name string) (*time.Location, error) {
	return nil, errNotImplemented
}

// Canonical returns t with the monotonic reading removed, converted to UTC.
// The result represents the same instant, can be compared with ==, and is
// safe as a map key.
func Canonical(t time.Time) time.Time {
	return time.Time{}
}

// Localize returns the instant whose wall-clock reading in loc equals the
// year, month, day, hour, minute, second, and nanosecond fields of wall.
// The zone of wall is ignored. A nil loc, or an unknown policy value, returns
// [ErrInvalid]. gap and overlap decide the nonexistent and ambiguous cases.
// The result is expressed in loc.
func Localize(wall time.Time, loc *time.Location, gap GapPolicy, overlap OverlapPolicy) (time.Time, error) {
	return time.Time{}, errNotImplemented
}
