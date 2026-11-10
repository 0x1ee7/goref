package timeutil_test

import (
	"errors"
	"testing"
	"time"

	timeutil "goref/exercises/17_time"
)

// mustSlot builds a slot from two instants, failing the test if it is invalid.
func mustSlot(t *testing.T, start, end time.Time) timeutil.TimeSlot {
	t.Helper()
	s, err := timeutil.NewTimeSlot(start, end)
	if err != nil {
		t.Fatalf("NewTimeSlot(%s, %s): %v", rfc(start), rfc(end), err)
	}
	return s
}

func slotStr(s timeutil.TimeSlot) string {
	return rfc(s.Start) + "/" + rfc(s.End)
}

func slotStrs(ss []timeutil.TimeSlot) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, slotStr(s))
	}
	return out
}

func TestPart6NewTimeSlot(t *testing.T) {
	berlin := zone(t, "Europe/Berlin")

	t.Run("valid slot keeps its bounds", func(t *testing.T) {
		start := local(t, berlin, "2026-06-01T11:00")
		end := local(t, berlin, "2026-06-01T12:00")
		s, err := timeutil.NewTimeSlot(start, end)
		if err != nil {
			t.Fatalf("NewTimeSlot error = %v", err)
		}
		if !s.Start.Equal(start) || !s.End.Equal(end) {
			t.Errorf("slot = %s, want %s/%s", slotStr(s), rfc(start), rfc(end))
		}
	})

	t.Run("empty slot is ErrInvalid", func(t *testing.T) {
		at := utc(t, "2026-06-01T10:00")
		if _, err := timeutil.NewTimeSlot(at, at); !errors.Is(err, timeutil.ErrInvalid) {
			t.Errorf("error = %v, want ErrInvalid", err)
		}
	})

	t.Run("inverted slot is ErrInvalid", func(t *testing.T) {
		if _, err := timeutil.NewTimeSlot(utc(t, "2026-06-01T11:00"), utc(t, "2026-06-01T10:00")); !errors.Is(err, timeutil.ErrInvalid) {
			t.Errorf("error = %v, want ErrInvalid", err)
		}
	})
}

func TestPart6Overlaps(t *testing.T) {
	berlin := zone(t, "Europe/Berlin")

	t.Run("partial overlap", func(t *testing.T) {
		a := mustSlot(t, utc(t, "2026-06-01T09:00"), utc(t, "2026-06-01T11:00"))
		b := mustSlot(t, utc(t, "2026-06-01T10:00"), utc(t, "2026-06-01T12:00"))
		if !a.Overlaps(b) || !b.Overlaps(a) {
			t.Error("partially overlapping slots should overlap in both directions")
		}
	})

	t.Run("containment", func(t *testing.T) {
		outer := mustSlot(t, utc(t, "2026-06-01T09:00"), utc(t, "2026-06-01T12:00"))
		inner := mustSlot(t, utc(t, "2026-06-01T10:00"), utc(t, "2026-06-01T11:00"))
		if !outer.Overlaps(inner) {
			t.Error("containing slot should overlap the inner slot")
		}
	})

	t.Run("cross-zone overlap compares instants", func(t *testing.T) {
		a := mustSlot(t, local(t, berlin, "2026-06-01T11:00"), local(t, berlin, "2026-06-01T12:00")) // 09:00Z-10:00Z
		b := mustSlot(t, utc(t, "2026-06-01T09:30"), utc(t, "2026-06-01T10:30"))
		if !a.Overlaps(b) {
			t.Error("Berlin 11:00-12:00 should overlap UTC 09:30-10:30")
		}
	})

	t.Run("touching slots do not overlap, but one nanosecond does", func(t *testing.T) {
		a := mustSlot(t, utc(t, "2026-06-01T09:00"), utc(t, "2026-06-01T10:00"))
		touching := mustSlot(t, utc(t, "2026-06-01T10:00"), utc(t, "2026-06-01T11:00"))
		oneNs := mustSlot(t, utc(t, "2026-06-01T09:59").Add(59*time.Second+999_999_999), utc(t, "2026-06-01T11:00"))
		if a.Overlaps(touching) || touching.Overlaps(a) {
			t.Error("touching slots must not overlap")
		}
		if !a.Overlaps(oneNs) {
			t.Error("slots sharing 1ns must overlap")
		}
	})

	t.Run("disjoint slots across zones do not overlap", func(t *testing.T) {
		a := mustSlot(t, local(t, berlin, "2026-06-01T13:00"), local(t, berlin, "2026-06-01T14:00")) // 11:00Z-12:00Z
		b := mustSlot(t, utc(t, "2026-06-01T09:00"), utc(t, "2026-06-01T10:00"))
		if a.Overlaps(b) || b.Overlaps(a) {
			t.Error("disjoint slots must not overlap")
		}
		if !a.Overlaps(a) {
			t.Error("a non-empty slot overlaps itself")
		}
	})
}

func TestPart6MergeSlots(t *testing.T) {
	berlin := zone(t, "Europe/Berlin")

	t.Run("merges overlapping and touching slots", func(t *testing.T) {
		in := []timeutil.TimeSlot{
			mustSlot(t, utc(t, "2026-06-01T10:00"), utc(t, "2026-06-01T11:00")),
			mustSlot(t, utc(t, "2026-06-01T09:00"), utc(t, "2026-06-01T10:00")),
			mustSlot(t, utc(t, "2026-06-01T10:30"), utc(t, "2026-06-01T12:00")),
		}
		got, err := timeutil.MergeSlots(in)
		if err != nil {
			t.Fatalf("MergeSlots error = %v", err)
		}
		if diff := cmpStrings([]string{"2026-06-01T09:00:00Z/2026-06-01T12:00:00Z"}, slotStrs(got)); diff != "" {
			t.Errorf("MergeSlots mismatch:\n%s", diff)
		}
	})

	t.Run("sorts across zones and returns UTC", func(t *testing.T) {
		in := []timeutil.TimeSlot{
			mustSlot(t, local(t, berlin, "2026-06-01T11:00"), local(t, berlin, "2026-06-01T12:00")), // 09:00Z-10:00Z
			mustSlot(t, utc(t, "2026-06-01T08:00"), utc(t, "2026-06-01T08:30")),
			mustSlot(t, utc(t, "2026-06-01T09:30"), utc(t, "2026-06-01T09:45")), // inside the Berlin slot
		}
		got, err := timeutil.MergeSlots(in)
		if err != nil {
			t.Fatalf("MergeSlots error = %v", err)
		}
		want := []string{"2026-06-01T08:00:00Z/2026-06-01T08:30:00Z", "2026-06-01T09:00:00Z/2026-06-01T10:00:00Z"}
		if diff := cmpStrings(want, slotStrs(got)); diff != "" {
			t.Errorf("MergeSlots mismatch:\n%s", diff)
		}
		for _, s := range got {
			if s.Start.Location() != time.UTC || s.End.Location() != time.UTC {
				t.Errorf("merged slot %s is not in UTC", slotStr(s))
			}
		}
	})

	t.Run("does not modify the input", func(t *testing.T) {
		in := []timeutil.TimeSlot{
			mustSlot(t, local(t, berlin, "2026-06-01T11:00"), local(t, berlin, "2026-06-01T12:00")),
		mustSlot(t, utc(t, "2026-06-01T07:00"), utc(t, "2026-06-01T08:00")),
		}
		got, err := timeutil.MergeSlots(in)
		if err != nil || len(got) != 2 {
			t.Fatalf("MergeSlots = %d slots, %v; want 2, nil", len(got), err)
		}
		if in[0].Start.Location().String() != "Europe/Berlin" || rfc(in[0].Start) != "2026-06-01T11:00:00+02:00" {
			t.Errorf("input[0] was modified: %s", slotStr(in[0]))
		}
		if rfc(in[1].Start) != "2026-06-01T07:00:00Z" {
			t.Errorf("input[1] was reordered: %s", slotStr(in[1]))
		}
	})

	t.Run("literal slot with End before Start is ErrInvalid", func(t *testing.T) {
		in := []timeutil.TimeSlot{{Start: utc(t, "2026-06-01T11:00"), End: utc(t, "2026-06-01T10:00")}}
		if _, err := timeutil.MergeSlots(in); !errors.Is(err, timeutil.ErrInvalid) {
			t.Errorf("error = %v, want ErrInvalid", err)
		}
	})
}

func TestPart6TotalBooked(t *testing.T) {
	berlin := zone(t, "Europe/Berlin")

	t.Run("overlapped time counts once", func(t *testing.T) {
		got, err := timeutil.TotalBooked([]timeutil.TimeSlot{
			mustSlot(t, utc(t, "2026-06-01T09:00"), utc(t, "2026-06-01T11:00")),
			mustSlot(t, utc(t, "2026-06-01T10:00"), utc(t, "2026-06-01T12:00")),
		})
		if err != nil {
			t.Fatalf("TotalBooked error = %v", err)
		}
		if got != 3*time.Hour {
			t.Errorf("TotalBooked = %v, want 3h0m0s", got)
		}
	})

	t.Run("cross-zone touching slots add up", func(t *testing.T) {
		got, err := timeutil.TotalBooked([]timeutil.TimeSlot{
			mustSlot(t, local(t, berlin, "2026-06-01T11:00"), local(t, berlin, "2026-06-01T12:00")), // 09:00Z-10:00Z
			mustSlot(t, utc(t, "2026-06-01T10:00"), utc(t, "2026-06-01T11:00")),
		})
		if err != nil {
			t.Fatalf("TotalBooked error = %v", err)
		}
		if got != 2*time.Hour {
			t.Errorf("TotalBooked = %v, want 2h0m0s", got)
		}
	})

	t.Run("spring DST day is 23 absolute hours", func(t *testing.T) {
		got, err := timeutil.TotalBooked([]timeutil.TimeSlot{
			mustSlot(t, local(t, berlin, "2026-03-28T12:00"), local(t, berlin, "2026-03-29T12:00")),
		})
		if err != nil {
			t.Fatalf("TotalBooked error = %v", err)
		}
		if got != 23*time.Hour {
			t.Errorf("TotalBooked = %v, want 23h0m0s", got)
		}
	})

	t.Run("invalid literal slot is ErrInvalid", func(t *testing.T) {
		_, err := timeutil.TotalBooked([]timeutil.TimeSlot{{Start: utc(t, "2026-06-01T11:00"), End: utc(t, "2026-06-01T10:00")}})
		if !errors.Is(err, timeutil.ErrInvalid) {
			t.Errorf("error = %v, want ErrInvalid", err)
		}
	})
}
