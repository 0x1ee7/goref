package signatures_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"goref/exercises/02_function_signatures"
)

var errNotify = errors.New("gateway down")

// recorder is a test-local Notifier that remembers every message.
type recorder struct {
	calls int
	msgs  []string
	err   error
}

func (r *recorder) Notify(_ context.Context, msg string) error {
	r.calls++
	r.msgs = append(r.msgs, msg)
	return r.err
}

func mustReminders(t *testing.T, clock signatures.Clock, n signatures.Notifier) *signatures.Reminders {
	t.Helper()
	r, err := signatures.NewReminders(clock, n)
	if err != nil {
		t.Fatalf("NewReminders: unexpected error: %v", err)
	}
	if r == nil {
		t.Fatal("NewReminders returned nil without error")
	}
	return r
}

func fixedClock(now time.Time) signatures.Clock {
	return signatures.ClockFunc(func() time.Time { return now })
}

var dueAt = time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

func TestPart4NewReminders(t *testing.T) {
	t.Run("valid dependencies", func(t *testing.T) {
		mustReminders(t, signatures.ClockFunc(time.Now), &recorder{})
	})
	t.Run("nil clock names the parameter", func(t *testing.T) {
		_, err := signatures.NewReminders(nil, &recorder{})
		if !errors.Is(err, signatures.ErrNilDependency) {
			t.Fatalf("error = %v, want errors.Is ErrNilDependency", err)
		}
		if !strings.Contains(err.Error(), "clock") {
			t.Errorf("error %q does not name the clock parameter", err)
		}
	})
	t.Run("nil notifier names the parameter", func(t *testing.T) {
		_, err := signatures.NewReminders(signatures.ClockFunc(time.Now), nil)
		if !errors.Is(err, signatures.ErrNilDependency) {
			t.Fatalf("error = %v, want errors.Is ErrNilDependency", err)
		}
		if !strings.Contains(err.Error(), "notifier") {
			t.Errorf("error %q does not name the notifier parameter", err)
		}
	})
}

type checkCase struct {
	name      string
	now       time.Time
	due       time.Time
	notifyErr error
	cancel    bool
	wantSent  bool
	wantCalls int
	wantMsg   string
	wantErr   error
}

func TestPart4Check(t *testing.T) {
	cases := []checkCase{
		{
			name:     "not yet due",
			now:      dueAt.Add(-time.Hour),
			due:      dueAt,
			wantSent: false,
		},
		{
			name:      "exact due time fires",
			now:       dueAt,
			due:       dueAt,
			wantSent:  true,
			wantCalls: 1,
			wantMsg:   "reminder due 2026-01-02T03:04:05Z",
		},
		{
			name:      "past due fires",
			now:       dueAt.Add(time.Hour),
			due:       dueAt,
			wantSent:  true,
			wantCalls: 1,
			wantMsg:   "reminder due 2026-01-02T03:04:05Z",
		},
		{
			name:      "due time is formatted in UTC",
			now:       dueAt,
			due:       time.Date(2026, time.January, 2, 5, 4, 5, 0, time.FixedZone("CEST", 2*3600)),
			wantSent:  true,
			wantCalls: 1,
			wantMsg:   "reminder due 2026-01-02T03:04:05Z",
		},
		{
			name:      "notifier failure means not sent",
			now:       dueAt.Add(time.Hour),
			due:       dueAt,
			notifyErr: errNotify,
			wantSent:  false,
			wantCalls: 1,
			wantMsg:   "reminder due 2026-01-02T03:04:05Z",
			wantErr:   errNotify,
		},
		{
			name:      "cancelled context returns before notifying",
			now:       dueAt.Add(time.Hour),
			due:       dueAt,
			cancel:    true,
			wantSent:  false,
			wantCalls: 0,
			wantErr:   context.Canceled,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{err: tc.notifyErr}
			r := mustReminders(t, fixedClock(tc.now), rec)

			ctx := t.Context()
			if tc.cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			sent, err := r.Check(ctx, tc.due)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("Check error = %v, want errors.Is %v", err, tc.wantErr)
				}
			} else if err != nil {
				t.Errorf("Check unexpected error: %v", err)
			}
			if sent != tc.wantSent {
				t.Errorf("Check sent = %v, want %v", sent, tc.wantSent)
			}
			if rec.calls != tc.wantCalls {
				t.Errorf("notifier calls = %d, want %d", rec.calls, tc.wantCalls)
			}
			if tc.wantMsg != "" && (len(rec.msgs) != 1 || rec.msgs[0] != tc.wantMsg) {
				t.Errorf("notifier messages = %q, want [%q]", rec.msgs, tc.wantMsg)
			}
		})
	}
}

func TestPart4ErrorPrefix(t *testing.T) {
	rec := &recorder{err: errNotify}
	r := mustReminders(t, fixedClock(dueAt), rec)

	_, err := r.Check(t.Context(), dueAt)
	if !errors.Is(err, errNotify) {
		t.Fatalf("Check error = %v, want errors.Is errNotify", err)
	}
	if !strings.HasPrefix(err.Error(), "reminders: ") {
		t.Errorf("error %q, want prefix %q", err, "reminders: ")
	}
}

func TestPart4MethodValues(t *testing.T) {
	rec := &recorder{}
	r := mustReminders(t, fixedClock(dueAt.Add(time.Minute)), signatures.NotifierFunc(rec.Notify))

	var check func(context.Context, time.Time) (bool, error) = r.Check
	sent, err := check(t.Context(), dueAt)
	if err != nil {
		t.Fatalf("Check unexpected error: %v", err)
	}
	if !sent {
		t.Errorf("Check sent = false, want true")
	}
	if len(rec.msgs) != 1 || rec.msgs[0] != "reminder due 2026-01-02T03:04:05Z" {
		t.Errorf("notifier messages = %q, want one reminder for 03:04:05Z", rec.msgs)
	}
}
