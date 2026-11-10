package timeutil_test

import (
	"testing"
	"time"

	timeutil "goref/exercises/17_time"
)

func TestPart3Humanize(t *testing.T) {
	berlin := zone(t, "Europe/Berlin")
	now := utc(t, "2026-06-01T12:00")

	cases := []struct {
		name string
		when time.Time
		want string
	}{
		{"same instant", now, "just now"},
		{"44s ago is still just now", now.Add(-44 * time.Second), "just now"},
		{"45s ago is one minute", now.Add(-45 * time.Second), "1 minute ago"},
		{"59 minutes ago", now.Add(-59 * time.Minute), "59 minutes ago"},
		{"one hour from now", now.Add(time.Hour), "in 1 hour"},
		{"3 hours ago", now.Add(-3 * time.Hour), "3 hours ago"},
		{"23 hours from now", now.Add(23 * time.Hour), "in 23 hours"},
		{"one day ago", now.Add(-24 * time.Hour), "1 day ago"},
		{"two days from now", now.Add(48 * time.Hour), "in 2 days"},
		{"29 days ago", now.Add(-29 * 24 * time.Hour), "29 days ago"},
		{"30 days ago switches to the date", now.Add(-30 * 24 * time.Hour), "on 2026-05-02"},
		{"far future uses the date", now.Add(60 * 24 * time.Hour), "on 2026-07-31"},
		{"date is read in the value's own zone", utc(t, "2026-05-01T23:30").In(berlin), "on 2026-05-02"},
		{"same instant in UTC gives the UTC date", utc(t, "2026-05-01T23:30"), "on 2026-05-01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := timeutil.Humanize(tc.when, now); got != tc.want {
				t.Errorf("Humanize(%s, %s) = %q, want %q", rfc(tc.when), rfc(now), got, tc.want)
			}
		})
	}

	t.Run("zone of now does not change the relative phrase", func(t *testing.T) {
		nowBerlin := now.In(berlin)
		if got := timeutil.Humanize(nowBerlin.Add(-3*time.Hour), nowBerlin); got != "3 hours ago" {
			t.Errorf("Humanize with Berlin now = %q, want \"3 hours ago\"", got)
		}
	})
}
