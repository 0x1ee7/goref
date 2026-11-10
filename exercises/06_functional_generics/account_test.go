package functional_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	fp "goref/exercises/06_functional_generics"
)

func openEvent() fp.Opened {
	return fp.Opened{AccountID: "acc-1", Owner: "Ada"}
}

func TestPart4Replay(t *testing.T) {
	t.Run("full lifecycle folds to the expected balance", func(t *testing.T) {
		events := []any{
			openEvent(),
			fp.Deposited{AmountCents: 1000},
			fp.Withdrawn{AmountCents: 300},
			fp.Frozen{},
			fp.Unfrozen{},
		}
		got, err := fp.Replay(events)
		if err != nil {
			t.Fatalf("Replay error: %v", err)
		}
		want := &fp.Account{AccountID: "acc-1", Owner: "Ada", BalanceCents: 700, Status: fp.StatusOpen}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Replay mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("freeze sets frozen status", func(t *testing.T) {
		got, err := fp.Replay([]any{openEvent(), fp.Frozen{}})
		if err != nil {
			t.Fatalf("Replay error: %v", err)
		}
		if got == nil || got.Status != fp.StatusFrozen {
			t.Errorf("Replay status = %+v, want frozen", got)
		}
	})
	t.Run("empty log is ErrEmptyLog", func(t *testing.T) {
		if _, err := fp.Replay(nil); !errors.Is(err, fp.ErrEmptyLog) {
			t.Errorf("Replay(nil) err = %v, want ErrEmptyLog", err)
		}
	})
}

func TestPart4ApplyErrors(t *testing.T) {
	open := openEvent()
	tests := []struct {
		name   string
		events []any
		want   error
	}{
		{"deposit before open", []any{fp.Deposited{AmountCents: 100}}, fp.ErrNotOpened},
		{"open twice", []any{open, open}, fp.ErrAlreadyOpened},
		{"zero deposit", []any{open, fp.Deposited{AmountCents: 0}}, fp.ErrNonPositiveAmount},
		{"negative withdrawal", []any{open, fp.Deposited{AmountCents: 100}, fp.Withdrawn{AmountCents: -5}}, fp.ErrNonPositiveAmount},
		{"overdraft", []any{open, fp.Deposited{AmountCents: 100}, fp.Withdrawn{AmountCents: 101}}, fp.ErrInsufficientFunds},
		{"deposit while frozen", []any{open, fp.Frozen{}, fp.Deposited{AmountCents: 5}}, fp.ErrAccountFrozen},
		{"withdraw while frozen", []any{open, fp.Deposited{AmountCents: 5}, fp.Frozen{}, fp.Withdrawn{AmountCents: 1}}, fp.ErrAccountFrozen},
		{"unknown event type", []any{open, "bogus"}, fp.ErrUnknownEvent},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := fp.Replay(tc.events); !errors.Is(err, tc.want) {
				t.Errorf("Replay err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestPart4Apply(t *testing.T) {
	t.Run("opens an account from nothing", func(t *testing.T) {
		got, err := fp.Apply(nil, fp.Opened{AccountID: "acc-9", Owner: "Grace"})
		if err != nil {
			t.Fatalf("Apply error: %v", err)
		}
		want := &fp.Account{AccountID: "acc-9", Owner: "Grace", Status: fp.StatusOpen}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Apply mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("returns a new value and leaves the input untouched", func(t *testing.T) {
		acc := &fp.Account{AccountID: "a", Owner: "o", BalanceCents: 100, Status: fp.StatusOpen}
		before := *acc
		got, err := fp.Apply(acc, fp.Deposited{AmountCents: 50})
		if err != nil {
			t.Fatalf("Apply error: %v", err)
		}
		if got == nil {
			t.Fatal("Apply returned nil account")
		}
		if got == acc {
			t.Error("Apply returned the input pointer; want a new account")
		}
		want := before
		want.BalanceCents = 150
		if diff := cmp.Diff(&want, got); diff != "" {
			t.Errorf("Apply mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(before, *acc); diff != "" {
			t.Errorf("Apply mutated its input (-before +after):\n%s", diff)
		}
	})
}

func TestPart4ReplayFrom(t *testing.T) {
	snap := func() *fp.Account {
		return &fp.Account{AccountID: "acc-1", Owner: "Ada", BalanceCents: 100, Status: fp.StatusOpen}
	}
	t.Run("resumes from a snapshot", func(t *testing.T) {
		got, err := fp.ReplayFrom(snap(), []any{fp.Deposited{AmountCents: 50}, fp.Withdrawn{AmountCents: 30}})
		if err != nil {
			t.Fatalf("ReplayFrom error: %v", err)
		}
		want := &fp.Account{AccountID: "acc-1", Owner: "Ada", BalanceCents: 120, Status: fp.StatusOpen}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("ReplayFrom mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("snapshot is not modified by replay", func(t *testing.T) {
		s := snap()
		got, err := fp.ReplayFrom(s, []any{fp.Deposited{AmountCents: 50}})
		if err != nil {
			t.Fatalf("ReplayFrom error: %v", err)
		}
		if got == nil || got.BalanceCents != 150 {
			t.Fatalf("ReplayFrom balance = %+v, want 150", got)
		}
		if diff := cmp.Diff(snap(), s); diff != "" {
			t.Errorf("snapshot changed (-want +got):\n%s", diff)
		}
	})
	t.Run("empty events return a copy of the snapshot", func(t *testing.T) {
		s := snap()
		got, err := fp.ReplayFrom(s, nil)
		if err != nil {
			t.Fatalf("ReplayFrom error: %v", err)
		}
		if got == nil {
			t.Fatal("ReplayFrom(snapshot, nil) returned nil")
		}
		if got == s {
			t.Error("ReplayFrom returned the snapshot pointer; want a copy")
		}
		if diff := cmp.Diff(snap(), got); diff != "" {
			t.Errorf("ReplayFrom mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("error index is relative to the events slice", func(t *testing.T) {
		_, err := fp.ReplayFrom(snap(), []any{fp.Deposited{AmountCents: 10}, fp.Withdrawn{AmountCents: 1000}})
		if err == nil || err.Error() != "event 1: insufficient funds" {
			t.Errorf("ReplayFrom err = %v, want \"event 1: insufficient funds\"", err)
		}
	})
}

func TestPart4Snapshots(t *testing.T) {
	t.Run("one snapshot per event", func(t *testing.T) {
		events := []any{openEvent(), fp.Deposited{AmountCents: 100}, fp.Withdrawn{AmountCents: 30}}
		snaps, err := fp.Snapshots(events)
		if err != nil {
			t.Fatalf("Snapshots error: %v", err)
		}
		if len(snaps) != 3 {
			t.Fatalf("Snapshots len = %d, want 3", len(snaps))
		}
		vals := make([]fp.Account, len(snaps))
		for i, s := range snaps {
			if s == nil {
				t.Fatalf("snapshot %d is nil", i)
			}
			vals[i] = *s
		}
		want := []fp.Account{
			{AccountID: "acc-1", Owner: "Ada", BalanceCents: 0, Status: fp.StatusOpen},
			{AccountID: "acc-1", Owner: "Ada", BalanceCents: 100, Status: fp.StatusOpen},
			{AccountID: "acc-1", Owner: "Ada", BalanceCents: 70, Status: fp.StatusOpen},
		}
		if diff := cmp.Diff(want, vals); diff != "" {
			t.Errorf("Snapshots mismatch (-want +got):\n%s", diff)
		}
		last, err := fp.Replay(events)
		if err != nil {
			t.Fatalf("Replay error: %v", err)
		}
		if diff := cmp.Diff(last, snaps[2]); diff != "" {
			t.Errorf("last snapshot differs from Replay (-replay +snapshot):\n%s", diff)
		}
	})
	t.Run("error names the failing event", func(t *testing.T) {
		_, err := fp.Snapshots([]any{openEvent(), fp.Withdrawn{AmountCents: 1}})
		if err == nil || err.Error() != "event 1: insufficient funds" {
			t.Errorf("Snapshots err = %v, want \"event 1: insufficient funds\"", err)
		}
	})
	t.Run("empty log yields non-nil empty slice", func(t *testing.T) {
		snaps, err := fp.Snapshots(nil)
		if err != nil {
			t.Fatalf("Snapshots(nil) error: %v", err)
		}
		if snaps == nil || len(snaps) != 0 {
			t.Errorf("Snapshots(nil) = %v, want non-nil empty slice", snaps)
		}
	})
}

func TestPart4DecideWithdraw(t *testing.T) {
	open := func() *fp.Account {
		return &fp.Account{AccountID: "a", Owner: "o", BalanceCents: 500, Status: fp.StatusOpen}
	}
	frozen := &fp.Account{AccountID: "a", Owner: "o", BalanceCents: 500, Status: fp.StatusFrozen}
	tests := []struct {
		name    string
		acc     *fp.Account
		amount  int64
		want    fp.Withdrawn
		wantErr error
	}{
		{"valid withdrawal returns the event", open(), 300, fp.Withdrawn{AmountCents: 300}, nil},
		{"insufficient funds", open(), 501, fp.Withdrawn{}, fp.ErrInsufficientFunds},
		{"frozen account", frozen, 1, fp.Withdrawn{}, fp.ErrAccountFrozen},
		{"non-positive amount", open(), 0, fp.Withdrawn{}, fp.ErrNonPositiveAmount},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := fp.DecideWithdraw(tc.acc, tc.amount)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("DecideWithdraw err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("DecideWithdraw error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("DecideWithdraw mismatch (-want +got):\n%s", diff)
			}
		})
	}
	t.Run("does not change the account", func(t *testing.T) {
		acc := open()
		got, err := fp.DecideWithdraw(acc, 300)
		if err != nil {
			t.Fatalf("DecideWithdraw error: %v", err)
		}
		if diff := cmp.Diff(fp.Withdrawn{AmountCents: 300}, got); diff != "" {
			t.Errorf("DecideWithdraw mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(open(), acc); diff != "" {
			t.Errorf("account changed (-want +got):\n%s", diff)
		}
	})
}
