// Part 4: calendars, month arithmetic, billing dates, and business days.
//
// Calendar functions take and return dates as time.Time values at UTC midnight.
// They read the wall-clock date of their argument in the argument's own zone.
package timeutil

import (
	"fmt"
	"time"
)

// monthLength returns the number of days in month of year, leap-year aware.
// month must already be validated.
func monthLength(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// DaysInMonth returns the number of days in month of year, leap-year aware.
// It returns [ErrInvalid] for a month outside January..December.
func DaysInMonth(year int, month time.Month) (int, error) {
	if month < time.January || month > time.December {
		return 0, fmt.Errorf("%w: month %d", ErrInvalid, int(month))
	}
	return monthLength(year, month), nil
}

// AddMonths adds calendar months to d's date and clamps the day to the target
// month's length: 2026-01-31 plus one month is 2026-02-28. [time.Time.AddDate]
// would give March 3 instead. Negative months are allowed. The result is UTC midnight.
func AddMonths(d time.Time, months int) time.Time {
	c := civilUTC(d)
	total := c.Year()*12 + int(c.Month()-1) + months
	year := floorDiv(total, 12)
	month := time.Month(total - year*12 + 1)
	day := min(c.Day(), monthLength(year, month))
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// floorDiv divides a by a positive b, rounding toward negative infinity.
func floorDiv(a, b int) int {
	q := a / b
	if a%b < 0 {
		q--
	}
	return q
}

// BillingDates returns count dates, each computed from anchor rather than from
// the previous date, so clamping never drifts. Anchor 2026-01-31 gives
// 01-31, 02-28, 03-31, 04-30. The first element is anchor's date. A count of 0
// returns an empty slice. A negative count returns [ErrInvalid].
func BillingDates(anchor time.Time, count int) ([]time.Time, error) {
	if count < 0 {
		return nil, fmt.Errorf("%w: negative count %d", ErrInvalid, count)
	}
	dates := make([]time.Time, 0, count)
	for i := range count {
		dates = append(dates, AddMonths(anchor, i))
	}
	return dates, nil
}

// dayKey is a civil date, independent of zone.
type dayKey struct {
	year  int
	month time.Month
	day   int
}

// keyOf returns the wall-clock date of t in t's own zone.
func keyOf(t time.Time) dayKey {
	return dayKey{year: t.Year(), month: t.Month(), day: t.Day()}
}

// BusinessCalendar answers business-day questions for a weekend set and a set
// of holidays. Build one with [NewBusinessCalendar]. The zero value is not
// usable. Dates are matched by wall-clock date in each value's own location.
type BusinessCalendar struct {
	weekend  [7]bool
	holidays map[dayKey]struct{}
}

// NewBusinessCalendar builds a calendar. Holidays are matched by civil date.
// If no weekend days are given, Saturday and Sunday are used. It returns
// [ErrInvalid] for a weekday outside Sunday..Saturday, or when every weekday
// is a weekend. Without at least one business weekday, AddBusinessDays would
// never terminate.
func NewBusinessCalendar(holidays []time.Time, weekend ...time.Weekday) (*BusinessCalendar, error) {
	if len(weekend) == 0 {
		weekend = []time.Weekday{time.Saturday, time.Sunday}
	}
	c := &BusinessCalendar{holidays: make(map[dayKey]struct{}, len(holidays))}
	days := 0
	for _, w := range weekend {
		if w < time.Sunday || w > time.Saturday {
			return nil, fmt.Errorf("%w: weekday %d", ErrInvalid, int(w))
		}
		if !c.weekend[w] {
			c.weekend[w] = true
			days++
		}
	}
	if days == len(c.weekend) {
		return nil, fmt.Errorf("%w: every weekday is a weekend", ErrInvalid)
	}
	for _, h := range holidays {
		c.holidays[keyOf(h)] = struct{}{}
	}
	return c, nil
}

// IsBusinessDay reports whether d is neither on the weekend nor a holiday.
func (c *BusinessCalendar) IsBusinessDay(d time.Time) bool {
	if c.weekend[d.Weekday()] {
		return false
	}
	_, holiday := c.holidays[keyOf(d)]
	return !holiday
}

// AddBusinessDays steps n business days forward, or back if n is negative.
// Each step lands on a business day, so a forward step from a weekend goes to
// the next business day. With n == 0 the result is d's date at UTC midnight.
func (c *BusinessCalendar) AddBusinessDays(d time.Time, n int) time.Time {
	day := civilUTC(d)
	step := 1
	if n < 0 {
		step = -1
	}
	for n != 0 {
		day = day.AddDate(0, 0, step)
		if c.IsBusinessDay(day) {
			n -= step
		}
	}
	return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
}

// BusinessDaysBetween counts the business days in (start, end]. If end is before
// start, it returns the negated count of the business days in (end, start].
func (c *BusinessCalendar) BusinessDaysBetween(start, end time.Time) int {
	if end.Before(start) {
		return -c.countAfter(end, start)
	}
	return c.countAfter(start, end)
}

// countAfter counts the business days in the civil-date range (from, to].
func (c *BusinessCalendar) countAfter(from, to time.Time) int {
	last := civilUTC(to)
	n := 0
	for day := civilUTC(from).AddDate(0, 0, 1); !day.After(last); day = day.AddDate(0, 0, 1) {
		if c.IsBusinessDay(day) {
			n++
		}
	}
	return n
}
