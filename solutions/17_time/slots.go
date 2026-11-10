// Part 6: booking slots across time zones.
package timeutil

import (
	"fmt"
	"slices"
	"time"
)

// TimeSlot is a half-open booking interval [Start, End). Both bounds are
// instants, so slots in different zones compare correctly.
type TimeSlot struct {
	Start, End time.Time
}

// NewTimeSlot returns a valid slot. It returns [ErrInvalid] unless end is
// strictly after start.
func NewTimeSlot(start, end time.Time) (TimeSlot, error) {
	if !end.After(start) {
		return TimeSlot{}, fmt.Errorf("%w: slot end %v is not after start %v", ErrInvalid, end, start)
	}
	return TimeSlot{Start: start, End: end}, nil
}

// Overlaps reports whether s and o share at least one instant. Touching slots,
// where s.End equals o.Start, do not overlap.
func (s TimeSlot) Overlaps(o TimeSlot) bool {
	return s.Start.Before(o.End) && o.Start.Before(s.End)
}

// MergeSlots returns the slots sorted by start, with overlapping or touching
// slots combined. The results are in UTC. The input slice is not modified. An
// invalid slot, such as a literal with End not after Start, returns [ErrInvalid].
func MergeSlots(slots []TimeSlot) ([]TimeSlot, error) {
	sorted := make([]TimeSlot, 0, len(slots))
	for _, s := range slots {
		if !s.End.After(s.Start) {
			return nil, fmt.Errorf("%w: slot [%v, %v) is empty or inverted", ErrInvalid, s.Start, s.End)
		}
		sorted = append(sorted, TimeSlot{Start: Canonical(s.Start), End: Canonical(s.End)})
	}
	slices.SortFunc(sorted, func(a, b TimeSlot) int { return a.Start.Compare(b.Start) })

	merged := sorted[:0]
	for _, s := range sorted {
		if n := len(merged); n > 0 && !s.Start.After(merged[n-1].End) {
			if s.End.After(merged[n-1].End) {
				merged[n-1].End = s.End
			}
			continue
		}
		merged = append(merged, s)
	}
	return merged, nil
}

// TotalBooked returns the covered time after merging, so overlapped time counts
// once. It returns [ErrInvalid] for an invalid slot and [ErrOverflow] if the
// total does not fit in a Duration.
func TotalBooked(slots []TimeSlot) (time.Duration, error) {
	merged, err := MergeSlots(slots)
	if err != nil {
		return 0, err
	}
	var total time.Duration
	for _, s := range merged {
		d, err := ElapsedBetween(s.Start, s.End)
		if err != nil {
			return 0, err
		}
		total, err = SafeAdd(total, d)
		if err != nil {
			return 0, err
		}
	}
	return total, nil
}
