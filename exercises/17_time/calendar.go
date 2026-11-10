// Part 4: calendars, month arithmetic, billing dates, and business days.
//
// Calendar functions take and return dates as time.Time values at UTC midnight.
// They read the wall-clock date of their argument in the argument's own zone.
package timeutil

import "time"

// DaysInMonth returns the number of days in month of year, leap-year aware.
// It returns [ErrInvalid] for a month outside January..December.
func DaysInMonth(year int, month time.Month) (int, error) {
	return 0, errNotImplemented
}

// AddMonths adds calendar months to d's date and clamps the day to the target
// month's length: 2026-01-31 plus one month is 2026-02-28. [time.Time.AddDate]
// would give March 3 instead. Negative months are allowed. The result is UTC midnight.
func AddMonths(d time.Time, months int) time.Time {
	return time.Time{}
}

// BillingDates returns count dates, each computed from anchor rather than from
// the previous date, so clamping never drifts. Anchor 2026-01-31 gives
// 01-31, 02-28, 03-31, 04-30. The first element is anchor's date. A count of 0
// returns an empty slice. A negative count returns [ErrInvalid].
func BillingDates(anchor time.Time, count int) ([]time.Time, error) {
	return nil, errNotImplemented
}

// BusinessCalendar answers business-day questions for a weekend set and a set
// of holidays. Build one with [NewBusinessCalendar]. The zero value is not
// usable. Dates are matched by wall-clock date in each value's own location.
type BusinessCalendar struct {
	// Engineer-owned: weekend set and holiday index.
}

// NewBusinessCalendar builds a calendar. Holidays are matched by civil date.
// If no weekend days are given, Saturday and Sunday are used. It returns
// [ErrInvalid] for a weekday outside Sunday..Saturday, or when every weekday
// is a weekend. Without at least one business weekday, AddBusinessDays would
// never terminate.
func NewBusinessCalendar(holidays []time.Time, weekend ...time.Weekday) (*BusinessCalendar, error) {
	return nil, errNotImplemented
}

// IsBusinessDay reports whether d is neither on the weekend nor a holiday.
func (c *BusinessCalendar) IsBusinessDay(d time.Time) bool {
	return false
}

// AddBusinessDays steps n business days forward, or back if n is negative.
// Each step lands on a business day, so a forward step from a weekend goes to
// the next business day. With n == 0 the result is d's date at UTC midnight.
func (c *BusinessCalendar) AddBusinessDays(d time.Time, n int) time.Time {
	return time.Time{}
}

// BusinessDaysBetween counts the business days in (start, end]. If end is before
// start, it returns the negated count of the business days in (end, start].
func (c *BusinessCalendar) BusinessDaysBetween(start, end time.Time) int {
	return 0
}
