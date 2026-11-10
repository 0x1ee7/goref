// Part 6: booking slots across time zones.
package timeutil

import "time"

// TimeSlot is a half-open booking interval [Start, End). Both bounds are
// instants, so slots in different zones compare correctly.
type TimeSlot struct {
	Start, End time.Time
}

// NewTimeSlot returns a valid slot. It returns [ErrInvalid] unless end is
// strictly after start.
func NewTimeSlot(start, end time.Time) (TimeSlot, error) {
	return TimeSlot{}, errNotImplemented
}

// Overlaps reports whether s and o share at least one instant. Touching slots,
// where s.End equals o.Start, do not overlap.
func (s TimeSlot) Overlaps(o TimeSlot) bool {
	return false
}

// MergeSlots returns the slots sorted by start, with overlapping or touching
// slots combined. The results are in UTC. The input slice is not modified. An
// invalid slot, such as a literal with End not after Start, returns [ErrInvalid].
func MergeSlots(slots []TimeSlot) ([]TimeSlot, error) {
	return nil, errNotImplemented
}

// TotalBooked returns the covered time after merging, so overlapped time counts
// once. It returns [ErrInvalid] for an invalid slot and [ErrOverflow] if the
// total does not fit in a Duration.
func TotalBooked(slots []TimeSlot) (time.Duration, error) {
	return 0, errNotImplemented
}
