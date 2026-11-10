package timeutil_test

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	timeutil "goref/exercises/17_time"
)

func TestPart4DaysInMonth(t *testing.T) {
	cases := []struct {
		year  int
		month time.Month
		want  int
	}{
		{2026, time.February, 28},
		{2024, time.February, 29},
		{2000, time.February, 29},
		{1900, time.February, 28},
		{2026, time.April, 30},
		{2026, time.December, 31},
	}
	for _, tc := range cases {
		t.Run(tc.month.String()+" "+strconv.Itoa(tc.year), func(t *testing.T) {
			got, err := timeutil.DaysInMonth(tc.year, tc.month)
			if err != nil {
				t.Fatalf("DaysInMonth(%d, %v) error = %v", tc.year, tc.month, err)
			}
			if got != tc.want {
				t.Errorf("DaysInMonth(%d, %v) = %d, want %d", tc.year, tc.month, got, tc.want)
			}
		})
	}

	t.Run("month outside 1..12 is ErrInvalid", func(t *testing.T) {
		for _, m := range []time.Month{0, 13} {
			if _, err := timeutil.DaysInMonth(2026, m); !errors.Is(err, timeutil.ErrInvalid) {
				t.Errorf("DaysInMonth(2026, %d) error = %v, want ErrInvalid", m, err)
			}
		}
	})
}

func TestPart4AddMonths(t *testing.T) {
	berlin := zone(t, "Europe/Berlin")
	cases := []struct {
		name   string
		start  string
		months int
		want   string
	}{
		{"clamps to Feb 28", "2026-01-31T00:00", 1, "2026-02-28T00:00:00Z"},
		{"clamps to Feb 29 in a leap year", "2024-01-31T00:00", 1, "2024-02-29T00:00:00Z"},
		{"two months skips a short month", "2026-01-31T00:00", 2, "2026-03-31T00:00:00Z"},
		{"negative clamps", "2026-03-31T00:00", -1, "2026-02-28T00:00:00Z"},
		{"year rollover", "2026-11-15T00:00", 3, "2027-02-15T00:00:00Z"},
		{"negative across years", "2026-02-15T00:00", -14, "2024-12-15T00:00:00Z"},
		{"zero months is identity", "2026-05-31T00:00", 0, "2026-05-31T00:00:00Z"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := timeutil.AddMonths(utc(t, tc.start), tc.months)
			if rfc(got) != tc.want {
				t.Errorf("AddMonths(%s, %d) = %s, want %s", tc.start, tc.months, rfc(got), tc.want)
			}
		})
	}

	t.Run("date fields are read in the value's zone, result is UTC midnight", func(t *testing.T) {
		// 2026-02-01 00:30 CET is 2026-01-31 23:30 UTC: the Berlin date is the one that counts.
		got := timeutil.AddMonths(local(t, berlin, "2026-02-01T00:30"), 1)
		if rfc(got) != "2026-03-01T00:00:00Z" {
			t.Errorf("AddMonths(Berlin 2026-02-01 00:30, 1) = %s, want 2026-03-01T00:00:00Z", rfc(got))
		}
	})

	t.Run("AddDate would overflow into March 3", func(t *testing.T) {
		if got := rfc(utc(t, "2026-01-31T00:00").AddDate(0, 1, 0)); got != "2026-03-03T00:00:00Z" {
			t.Fatalf("precondition: AddDate(0,1,0) = %s", got)
		}
		if got := rfc(timeutil.AddMonths(utc(t, "2026-01-31T00:00"), 1)); got != "2026-02-28T00:00:00Z" {
			t.Errorf("AddMonths(2026-01-31, 1) = %s, want 2026-02-28T00:00:00Z", got)
		}
	})
}

func TestPart4BillingDates(t *testing.T) {
	cases := []struct {
		name   string
		anchor string
		count  int
		want   []string
	}{
		{"no drift after a short month", "2026-01-31T00:00", 4,
			[]string{"2026-01-31T00:00:00Z", "2026-02-28T00:00:00Z", "2026-03-31T00:00:00Z", "2026-04-30T00:00:00Z"}},
		{"leap-day anchor", "2024-02-29T00:00", 3,
			[]string{"2024-02-29T00:00:00Z", "2024-03-29T00:00:00Z", "2024-04-29T00:00:00Z"}},
		{"year end", "2026-11-30T00:00", 3,
			[]string{"2026-11-30T00:00:00Z", "2026-12-30T00:00:00Z", "2027-01-30T00:00:00Z"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := timeutil.BillingDates(utc(t, tc.anchor), tc.count)
			if err != nil {
				t.Fatalf("BillingDates error = %v", err)
			}
			if diffRFC(tc.want, got) != "" {
				t.Errorf("BillingDates mismatch:\n%s", diffRFC(tc.want, got))
			}
		})
	}

	t.Run("count 0 is empty and negative count is ErrInvalid", func(t *testing.T) {
		got, err := timeutil.BillingDates(utc(t, "2026-01-31T00:00"), 0)
		if err != nil || len(got) != 0 {
			t.Errorf("BillingDates(count 0) = %v, %v; want empty, nil", got, err)
		}
		if _, err := timeutil.BillingDates(utc(t, "2026-01-31T00:00"), -1); !errors.Is(err, timeutil.ErrInvalid) {
			t.Errorf("BillingDates(count -1) error = %v, want ErrInvalid", err)
		}
	})
}

func TestPart4BusinessCalendar(t *testing.T) {
	// 2026-01-01 is a Thursday and 2026-12-25 a Friday.
	holidays := []time.Time{utc(t, "2026-01-01T00:00"), utc(t, "2026-12-25T00:00")}
	cal := newCal(t, holidays)

	t.Run("constructor rejects a calendar with no business days", func(t *testing.T) {
		_, err := timeutil.NewBusinessCalendar(nil,
			time.Sunday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday)
		if !errors.Is(err, timeutil.ErrInvalid) {
			t.Errorf("error = %v, want ErrInvalid", err)
		}
	})

	t.Run("constructor rejects a weekday outside Sunday..Saturday", func(t *testing.T) {
		_, err := timeutil.NewBusinessCalendar(nil, time.Weekday(7))
		if !errors.Is(err, timeutil.ErrInvalid) {
			t.Errorf("error = %v, want ErrInvalid", err)
		}
	})

	t.Run("weekend and holidays are not business days", func(t *testing.T) {
		if !cal.IsBusinessDay(utc(t, "2026-06-05T00:00")) {
			t.Error("Friday 2026-06-05 should be a business day")
		}
		for _, d := range []string{"2026-06-06T00:00", "2026-06-07T00:00", "2026-12-25T00:00"} {
			if cal.IsBusinessDay(utc(t, d)) {
				t.Errorf("IsBusinessDay(%s) = true, want false", d)
			}
		}
	})

	t.Run("custom weekend of Friday and Saturday", func(t *testing.T) {
		custom := newCal(t, nil, time.Friday, time.Saturday)
		if !custom.IsBusinessDay(utc(t, "2026-06-07T00:00")) {
			t.Error("Sunday should be a business day with a Fri/Sat weekend")
		}
		if custom.IsBusinessDay(utc(t, "2026-06-05T00:00")) {
			t.Error("Friday should be a weekend day with a Fri/Sat weekend")
		}
	})

	t.Run("AddBusinessDays", func(t *testing.T) {
		cases := []struct {
			name  string
			start string
			n     int
			want  string
		}{
			{"forward over weekend", "2026-06-05T00:00", 1, "2026-06-08T00:00:00Z"},
			{"from a Saturday, first step is the next business day", "2026-06-06T00:00", 1, "2026-06-08T00:00:00Z"},
			{"backward over weekend", "2026-06-08T00:00", -1, "2026-06-05T00:00:00Z"},
			{"backward from a Saturday", "2026-06-06T00:00", -1, "2026-06-05T00:00:00Z"},
			{"five business days from Monday", "2026-06-01T00:00", 5, "2026-06-08T00:00:00Z"},
			{"ten business days from Monday", "2026-06-01T00:00", 10, "2026-06-15T00:00:00Z"},
			{"forward skips the Christmas holiday", "2026-12-24T00:00", 1, "2026-12-28T00:00:00Z"},
			{"backward skips the Christmas holiday", "2026-12-28T00:00", -1, "2026-12-24T00:00:00Z"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got := cal.AddBusinessDays(utc(t, tc.start), tc.n)
				if rfc(got) != tc.want {
					t.Errorf("AddBusinessDays(%s, %d) = %s, want %s", tc.start, tc.n, rfc(got), tc.want)
				}
			})
		}
	})

	t.Run("AddBusinessDays uses the wall date and returns UTC midnight", func(t *testing.T) {
		berlin := zone(t, "Europe/Berlin")
		fri := local(t, berlin, "2026-06-05T15:00") // Friday 15:00 in Berlin
		if got := rfc(cal.AddBusinessDays(fri, 0)); got != "2026-06-05T00:00:00Z" {
			t.Errorf("AddBusinessDays(fri, 0) = %s, want 2026-06-05T00:00:00Z", got)
		}
		if got := rfc(cal.AddBusinessDays(fri, 1)); got != "2026-06-08T00:00:00Z" {
			t.Errorf("AddBusinessDays(fri, 1) = %s, want 2026-06-08T00:00:00Z", got)
		}
	})

	t.Run("BusinessDaysBetween", func(t *testing.T) {
		cases := []struct {
			name       string
			start, end string
			want       int
		}{
			{"Monday to Friday counts Tue..Fri", "2026-06-01T00:00", "2026-06-05T00:00", 4},
			{"Friday to next Monday counts Monday", "2026-06-05T00:00", "2026-06-08T00:00", 1},
			{"from a Saturday counts the Monday", "2026-06-06T00:00", "2026-06-08T00:00", 1},
			{"full business week", "2026-06-01T00:00", "2026-06-08T00:00", 5},
			{"holiday in the range is skipped", "2026-12-24T00:00", "2026-12-28T00:00", 1},
			{"reversed gives the negated count", "2026-06-05T00:00", "2026-06-01T00:00", -4},
			{"reversed one day", "2026-06-08T00:00", "2026-06-05T00:00", -1},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := cal.BusinessDaysBetween(utc(t, tc.start), utc(t, tc.end)); got != tc.want {
					t.Errorf("BusinessDaysBetween(%s, %s) = %d, want %d", tc.start, tc.end, got, tc.want)
				}
			})
		}
	})
}

// newCal builds a calendar or fails the subtest.
func newCal(t *testing.T, holidays []time.Time, weekend ...time.Weekday) *timeutil.BusinessCalendar {
	t.Helper()
	c, err := timeutil.NewBusinessCalendar(holidays, weekend...)
	if err != nil {
		t.Fatalf("NewBusinessCalendar: %v", err)
	}
	return c
}

// diffRFC compares the RFC 3339 renderings of got against want.
func diffRFC(want []string, got []time.Time) string {
	return cmp.Diff(want, rfcs(got))
}
