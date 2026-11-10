package support_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	support "goref/exercises/09_errors_and_cleanup"
)

func TestPart7Guard(t *testing.T) {
	t.Run("plain error is returned unchanged", func(t *testing.T) {
		boom := errors.New("boom")
		err := support.Guard(func() error { return boom })
		if err != boom {
			t.Fatalf("Guard() = %v, want the error unchanged", err)
		}
	})

	t.Run("success and error both pass through", func(t *testing.T) {
		boom := errors.New("boom")
		if err := support.Guard(func() error { return boom }); !errors.Is(err, boom) {
			t.Fatalf("Guard(error) = %v, want boom", err)
		}
		if err := support.Guard(func() error { return nil }); err != nil {
			t.Errorf("Guard(nil) = %v, want nil", err)
		}
	})

	t.Run("string panic becomes PanicError with a stack", func(t *testing.T) {
		err := support.Guard(func() error { panic("kaboom") })
		pe, ok := errors.AsType[*support.PanicError](err)
		if !ok {
			t.Fatalf("Guard(panic) = %v (%T), want *PanicError", err, err)
		}
		if pe.Value != "kaboom" {
			t.Errorf("Value = %v, want %q", pe.Value, "kaboom")
		}
		if !bytes.Contains(pe.Stack, []byte("goroutine")) {
			t.Errorf("Stack = %q, want a goroutine stack trace", pe.Stack)
		}
	})

	t.Run("error panic value is reachable through Unwrap", func(t *testing.T) {
		boom := errors.New("boom")
		err := support.Guard(func() error { panic(boom) })
		if !errors.Is(err, boom) {
			t.Errorf("errors.Is(Guard(panic(err)), boom) = false, want true")
		}
	})
}

func TestPart7Must(t *testing.T) {
	t.Run("returns the value when err is nil", func(t *testing.T) {
		if got := support.Must(42, nil); got != 42 {
			t.Errorf("Must(42, nil) = %d, want 42", got)
		}
	})

	t.Run("panics with the error", func(t *testing.T) {
		boom := errors.New("bad pattern")
		var recovered any
		func() {
			defer func() { recovered = recover() }()
			_ = support.Must(0, boom)
		}()
		err, ok := recovered.(error)
		if !ok {
			t.Fatalf("recovered = %v (%T), want the error as the panic value", recovered, recovered)
		}
		if !errors.Is(err, boom) {
			t.Errorf("recovered error = %v, want boom", err)
		}
	})
}

func TestPart7RunJob(t *testing.T) {
	t.Run("success returns nil after the job ran with ctx", func(t *testing.T) {
		ran := false
		err := support.RunJob(t.Context(), func(ctx context.Context) error {
			ran = true
			if ctx == nil {
				return errors.New("nil context")
			}
			return nil
		})
		if err != nil {
			t.Errorf("RunJob() = %v, want nil", err)
		}
		if !ran {
			t.Error("job did not run")
		}
	})

	t.Run("job error without cancellation is unchanged", func(t *testing.T) {
		boom := errors.New("boom")
		err := support.RunJob(t.Context(), func(context.Context) error { return boom })
		if err != boom {
			t.Errorf("RunJob() = %v, want the job error unchanged", err)
		}
	})

	t.Run("cancelled context adds the cause", func(t *testing.T) {
		cause := errors.New("shutdown")
		ctx, cancel := context.WithCancelCause(t.Context())
		cancel(cause)
		err := support.RunJob(ctx, func(ctx context.Context) error { return ctx.Err() })
		if !errors.Is(err, context.Canceled) {
			t.Errorf("RunJob() = %v, want it to still match context.Canceled", err)
		}
		if !errors.Is(err, cause) {
			t.Errorf("RunJob() = %v, want it to match the cancel cause", err)
		}
	})

	t.Run("job error that already matches the cause is unchanged", func(t *testing.T) {
		cause := errors.New("shutdown")
		ctx, cancel := context.WithCancelCause(t.Context())
		cancel(cause)
		err := support.RunJob(ctx, func(context.Context) error { return cause })
		if err != cause {
			t.Errorf("RunJob() = %v, want the job error unchanged (no duplicate cause)", err)
		}
	})

	t.Run("panic in job propagates", func(t *testing.T) {
		var recovered any
		func() {
			defer func() { recovered = recover() }()
			_ = support.RunJob(t.Context(), func(context.Context) error { panic("job panic") })
		}()
		if recovered != "job panic" {
			t.Errorf("recovered = %v, want %q", recovered, "job panic")
		}
	})
}
