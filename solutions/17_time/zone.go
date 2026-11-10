// Part 1: instants, zones, and DST wall-clock policy.
package timeutil

import (
	"fmt"
	"slices"
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
	if name == "" || name == "Local" {
		return nil, fmt.Errorf("%w: %q depends on the machine", ErrUnknownZone, name)
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnknownZone, err)
	}
	return loc, nil
}

// Canonical returns t with the monotonic reading removed, converted to UTC.
// The result represents the same instant, can be compared with ==, and is
// safe as a map key.
func Canonical(t time.Time) time.Time {
	return t.Round(0).UTC()
}

// Localize returns the instant whose wall-clock reading in loc equals the
// year, month, day, hour, minute, second, and nanosecond fields of wall.
// The zone of wall is ignored. A nil loc, or an unknown policy value, returns
// [ErrInvalid]. gap and overlap decide the nonexistent and ambiguous cases.
// The result is expressed in loc.
func Localize(wall time.Time, loc *time.Location, gap GapPolicy, overlap OverlapPolicy) (time.Time, error) {
	if loc == nil {
		return time.Time{}, fmt.Errorf("%w: nil location", ErrInvalid)
	}
	if err := checkPolicies(gap, overlap); err != nil {
		return time.Time{}, err
	}
	return resolve(wall, loc, gap, overlap)
}

const oneDay = 24 * time.Hour

// civilUTC returns the wall-clock fields of t as a UTC time, discarding t's zone.
func civilUTC(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
}

func checkPolicies(gap GapPolicy, overlap OverlapPolicy) error {
	switch gap {
	case GapShiftForward, GapSkip:
	default:
		return fmt.Errorf("%w: gap policy %d", ErrInvalid, int(gap))
	}
	switch overlap {
	case OverlapEarlier, OverlapLater, OverlapSkip:
	default:
		return fmt.Errorf("%w: overlap policy %d", ErrInvalid, int(overlap))
	}
	return nil
}

// resolve maps a wall-clock reading in loc to an instant. The offsets in force
// a day before and a day after the reading are the only ones that can fit it,
// so they are tried as candidates. Each candidate is kept if it reads back as
// the same wall-clock time.
func resolve(wall time.Time, loc *time.Location, gap GapPolicy, overlap OverlapPolicy) (time.Time, error) {
	w := civilUTC(wall)
	_, before := w.Add(-oneDay).In(loc).Zone()
	_, after := w.Add(oneDay).In(loc).Zone()

	var cands []time.Time
	for _, off := range []int{before, after} {
		u := w.Add(-time.Duration(off) * time.Second)
		if !civilUTC(u.In(loc)).Equal(w) {
			continue
		}
		if slices.ContainsFunc(cands, func(c time.Time) bool { return c.Equal(u) }) {
			continue
		}
		cands = append(cands, u)
	}

	switch len(cands) {
	case 0:
		if gap == GapSkip {
			return time.Time{}, fmt.Errorf("%w: %s in %s", ErrNonexistent, w.Format(time.DateTime), loc)
		}
		// The clock jumps forward: the pre-transition offset maps the reading
		// past the gap.
		return w.Add(-time.Duration(before) * time.Second).In(loc), nil
	case 1:
		return cands[0].In(loc), nil
	}

	slices.SortFunc(cands, func(a, b time.Time) int { return a.Compare(b) })
	switch overlap {
	case OverlapLater:
		return cands[1].In(loc), nil
	case OverlapSkip:
		return time.Time{}, fmt.Errorf("%w: %s in %s", ErrAmbiguous, w.Format(time.DateTime), loc)
	default:
		return cands[0].In(loc), nil
	}
}
