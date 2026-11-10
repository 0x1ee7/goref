package goroutines_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"goref/exercises/14_goroutines_and_sync"
)

func noopJob(context.Context) error { return nil }

func TestPart7AddValidation(t *testing.T) {
	tests := []struct {
		name     string
		jobName  string
		interval time.Duration
		job      goroutines.Job
		wantErr  error
	}{
		{name: "zero interval", jobName: "a", interval: 0, job: noopJob, wantErr: goroutines.ErrInvalidInterval},
		{name: "negative interval", jobName: "a", interval: -time.Second, job: noopJob, wantErr: goroutines.ErrInvalidInterval},
		{name: "nil job", jobName: "a", interval: time.Second, job: nil, wantErr: goroutines.ErrNilJob},
		{name: "valid job", jobName: "a", interval: time.Second, job: noopJob, wantErr: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s goroutines.Scheduler
			err := s.Add(tt.jobName, tt.interval, tt.job)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Add() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestPart7AddDuplicateAndAfterStart(t *testing.T) {
	var s goroutines.Scheduler
	if err := s.Add("sync", time.Hour, noopJob); err != nil {
		t.Fatalf("Add(sync) error = %v", err)
	}
	if err := s.Add("sync", time.Hour, noopJob); !errors.Is(err, goroutines.ErrDuplicateJob) {
		t.Errorf("duplicate Add() error = %v, want ErrDuplicateJob", err)
	}
	if err := s.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = s.Stop(context.Background())
	})
	if err := s.Add("late", time.Hour, noopJob); !errors.Is(err, goroutines.ErrStarted) {
		t.Errorf("Add() after Start error = %v, want ErrStarted", err)
	}
}

func TestPart7LifecycleErrors(t *testing.T) {
	t.Run("double Start", func(t *testing.T) {
		var s goroutines.Scheduler
		if err := s.Add("j", time.Hour, noopJob); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if err := s.Start(t.Context()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		t.Cleanup(func() { _ = s.Stop(context.Background()) })
		if err := s.Start(t.Context()); !errors.Is(err, goroutines.ErrStarted) {
			t.Errorf("second Start() error = %v, want ErrStarted", err)
		}
	})
	t.Run("Start after Stop", func(t *testing.T) {
		var s goroutines.Scheduler
		if err := s.Add("j", time.Hour, noopJob); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if err := s.Start(t.Context()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if err := s.Stop(t.Context()); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		if err := s.Start(t.Context()); !errors.Is(err, goroutines.ErrStopped) {
			t.Errorf("Start() after Stop error = %v, want ErrStopped", err)
		}
	})
	t.Run("Stop before Start and repeated Stop are no-ops", func(t *testing.T) {
		var s goroutines.Scheduler
		if err := s.Stop(t.Context()); err != nil {
			t.Errorf("Stop() before Start error = %v, want nil", err)
		}
		if err := s.Add("j", time.Hour, noopJob); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if err := s.Start(t.Context()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if err := s.Stop(t.Context()); err != nil {
			t.Fatalf("first Stop() error = %v", err)
		}
		if err := s.Stop(t.Context()); err != nil {
			t.Errorf("second Stop() error = %v, want nil", err)
		}
	})
}

func TestPart7RunsImmediatelyThenEachInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var s goroutines.Scheduler
		var runs atomic.Int64
		if err := s.Add("tick", 10*time.Second, func(context.Context) error {
			runs.Add(1)
			return nil
		}); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if err := s.Start(t.Context()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		synctest.Wait()
		if n := runs.Load(); n != 1 {
			t.Fatalf("runs right after Start = %d, want 1 (job runs immediately)", n)
		}
		time.Sleep(25 * time.Second) // fake clock: runs at t=0, 10, 20
		if n := runs.Load(); n != 3 {
			t.Errorf("runs at t=25s = %d, want 3", n)
		}
		if err := s.Stop(t.Context()); err != nil {
			t.Errorf("Stop() error = %v", err)
		}
	})
}

func TestPart7IntervalIsAfterRunEnds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var s goroutines.Scheduler
		start := time.Now()
		var mu sync.Mutex
		var starts []time.Duration
		if err := s.Add("slow", 10*time.Second, func(ctx context.Context) error {
			mu.Lock()
			starts = append(starts, time.Since(start))
			mu.Unlock()
			select {
			case <-time.After(4 * time.Second):
			case <-ctx.Done():
			}
			return nil
		}); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if err := s.Start(t.Context()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		time.Sleep(29 * time.Second)
		synctest.Wait()
		mu.Lock()
		got := append([]time.Duration(nil), starts...)
		mu.Unlock()
		// Run 1 takes 4s, then waits 10s: starts at 0, 14, 28.
		want := []time.Duration{0, 14 * time.Second, 28 * time.Second}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("run start offsets mismatch (-want +got):\n%s", diff)
		}
		if err := s.Stop(t.Context()); err != nil {
			t.Errorf("Stop() error = %v", err)
		}
	})
}

func TestPart7JobsAreIndependent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var s goroutines.Scheduler
		var fast, slow atomic.Int64
		if err := s.Add("fast", 5*time.Second, func(context.Context) error {
			fast.Add(1)
			return nil
		}); err != nil {
			t.Fatalf("Add(fast) error = %v", err)
		}
		if err := s.Add("slow", 10*time.Second, func(context.Context) error {
			slow.Add(1)
			return nil
		}); err != nil {
			t.Fatalf("Add(slow) error = %v", err)
		}
		if err := s.Start(t.Context()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		time.Sleep(12 * time.Second)
		// fast: t=0,5,10 ; slow: t=0,10. Checked away from the boundaries.
		if f, sl := fast.Load(), slow.Load(); f != 3 || sl != 2 {
			t.Errorf("runs at t=12s: fast=%d slow=%d, want fast=3 slow=2", f, sl)
		}
		if err := s.Stop(t.Context()); err != nil {
			t.Errorf("Stop() error = %v", err)
		}
	})
}

func TestPart7FailuresRecorded(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var s goroutines.Scheduler
		if err := s.Add("flaky", 5*time.Second, func(context.Context) error {
			return errBoom
		}); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if err := s.Start(t.Context()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		time.Sleep(11 * time.Second) // runs at t=0, 5, 10
		if err := s.Stop(t.Context()); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		errs := s.Errors()
		if len(errs) != 3 {
			t.Fatalf("len(Errors()) = %d, want 3", len(errs))
		}
		for _, e := range errs {
			if e.Name != "flaky" || !errors.Is(e, errBoom) {
				t.Errorf("JobError = %+v, want Name flaky wrapping errBoom", e)
			}
		}
		// Errors returns a copy.
		errs[0].Name = "mutated"
		if got := s.Errors()[0].Name; got != "flaky" {
			t.Errorf("Errors() returned shared storage; second call Name = %q", got)
		}
	})
}

func TestPart7JobErrorFormatting(t *testing.T) {
	e := goroutines.JobError{Name: "sync", Err: errBoom}
	if got, want := e.Error(), "job sync: boom"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if !errors.Is(e, errBoom) {
		t.Errorf("errors.Is(JobError, errBoom) = false, want true (Unwrap)")
	}
}

func TestPart7CancellationNotRecorded(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var s goroutines.Scheduler
		started := make(chan struct{}, 1)
		if err := s.Add("waiter", time.Hour, func(ctx context.Context) error {
			started <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		}); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if err := s.Start(t.Context()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		recvOrFail(t, started, "job start")
		if err := s.Stop(t.Context()); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		if n := len(s.Errors()); n != 0 {
			t.Errorf("len(Errors()) = %d, want 0 (cancellation during Stop is not a failure)", n)
		}
	})
}

func TestPart7StopHonoursContextAndKeepsWaiting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		var finished atomic.Bool
		var s goroutines.Scheduler
		if err := s.Add("stubborn", time.Hour, func(ctx context.Context) error {
			select {
			case <-release:
			case <-time.After(time.Minute):
			}
			finished.Store(true)
			return nil
		}); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if err := s.Start(t.Context()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		synctest.Wait()

		sctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		if err := s.Stop(sctx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Stop() with short deadline error = %v, want DeadlineExceeded", err)
		}

		close(release)
		if err := s.Stop(t.Context()); err != nil {
			t.Errorf("later Stop() error = %v, want nil once the job exits", err)
		}
		if !finished.Load() {
			t.Errorf("job had not finished when Stop returned nil")
		}
	})
}

func TestPart7NoRunsAfterStop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var s goroutines.Scheduler
		var runs atomic.Int64
		if err := s.Add("tick", 2*time.Second, func(context.Context) error {
			runs.Add(1)
			return nil
		}); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if err := s.Start(t.Context()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		time.Sleep(3 * time.Second)
		if err := s.Stop(t.Context()); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		before := runs.Load()
		time.Sleep(30 * time.Second)
		if after := runs.Load(); after != before {
			t.Errorf("job ran %d more times after Stop returned, want 0", after-before)
		}
	})
}
