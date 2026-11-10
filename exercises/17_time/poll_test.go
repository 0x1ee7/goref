package timeutil_test

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"

	timeutil "goref/exercises/17_time"
)

func TestPart7Poll(t *testing.T) {
	t.Run("calls fn on each tick until the context is cancelled", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			start := time.Now()
			var offsets []time.Duration
			err := timeutil.Poll(ctx, time.Second, func(ctx context.Context, tick time.Time) error {
				offsets = append(offsets, tick.Sub(start))
				if len(offsets) == 3 {
					cancel()
				}
				return nil
			})
			if !errors.Is(err, context.Canceled) {
				t.Errorf("Poll error = %v, want context.Canceled", err)
			}
			want := []time.Duration{time.Second, 2 * time.Second, 3 * time.Second}
			if diff := cmp.Diff(want, offsets); diff != "" {
				t.Errorf("tick offsets mismatch (-want +got):\n%s", diff)
			}
		})
	})

	t.Run("fn error stops the loop and is returned unchanged", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			errBoom := errors.New("boom")
			calls := 0
			err := timeutil.Poll(t.Context(), time.Second, func(ctx context.Context, tick time.Time) error {
				calls++
				if calls == 2 {
					return errBoom
				}
				return nil
			})
			if !errors.Is(err, errBoom) {
				t.Errorf("Poll error = %v, want errBoom", err)
			}
			if calls != 2 {
				t.Errorf("fn calls = %d, want 2", calls)
			}
		})
	})

	t.Run("slow fn drops ticks instead of bursting", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			errStop := errors.New("stop after four calls")
			var began []time.Duration
			err := timeutil.Poll(t.Context(), time.Second, func(ctx context.Context, tick time.Time) error {
				began = append(began, time.Since(start))
				time.Sleep(2500 * time.Millisecond)
				if len(began) == 4 {
					return errStop
				}
				return nil
			})
			if !errors.Is(err, errStop) {
				t.Errorf("Poll error = %v, want errStop", err)
			}
			want := []time.Duration{
				1 * time.Second,
				3500 * time.Millisecond,
				6 * time.Second,
				8500 * time.Millisecond,
			}
			if diff := cmp.Diff(want, began); diff != "" {
				t.Errorf("fn start offsets mismatch (-want +got):\n%s", diff)
			}
		})
	})

	t.Run("non-positive interval and nil fn are ErrInvalid, not a panic", func(t *testing.T) {
		fn := func(context.Context, time.Time) error { return nil }
		if err := timeutil.Poll(context.Background(), 0, fn); !errors.Is(err, timeutil.ErrInvalid) {
			t.Errorf("zero interval error = %v, want ErrInvalid", err)
		}
		if err := timeutil.Poll(context.Background(), -time.Second, fn); !errors.Is(err, timeutil.ErrInvalid) {
			t.Errorf("negative interval error = %v, want ErrInvalid", err)
		}
		if err := timeutil.Poll(context.Background(), time.Second, nil); !errors.Is(err, timeutil.ErrInvalid) {
			t.Errorf("nil fn error = %v, want ErrInvalid", err)
		}
	})

	t.Run("already cancelled context returns without calling fn", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		calls := 0
		err := timeutil.Poll(ctx, time.Hour, func(context.Context, time.Time) error {
			calls++
			return nil
		})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Poll error = %v, want context.Canceled", err)
		}
		if calls != 0 {
			t.Errorf("fn calls = %d, want 0", calls)
		}
	})
}
