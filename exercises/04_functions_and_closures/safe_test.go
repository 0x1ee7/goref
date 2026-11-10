package funcs_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	funcs "goref/exercises/04_functions_and_closures"
)

func TestPart3Protect(t *testing.T) {
	t.Run("returns the function's error unchanged, and nil on success", func(t *testing.T) {
		sentinel := errors.New("sentinel")
		if err := funcs.Protect(func() error { return sentinel }); err != sentinel {
			t.Errorf("Protect = %v, want the sentinel error itself", err)
		}
		if err := funcs.Protect(func() error { return nil }); err != nil {
			t.Errorf("Protect on success = %v, want nil", err)
		}
	})

	t.Run("string panic becomes a PanicError carrying a stack", func(t *testing.T) {
		err := funcs.Protect(func() error { panic("boom") })
		var pe *funcs.PanicError
		if !errors.As(err, &pe) {
			t.Fatalf("err = %v (%T), want *funcs.PanicError", err, err)
		}
		if s, _ := pe.Value.(string); s != "boom" {
			t.Errorf("Value = %v, want \"boom\"", pe.Value)
		}
		if !strings.Contains(string(pe.Stack), "TestPart3Protect") {
			t.Errorf("Stack does not mention the panicking function:\n%s", pe.Stack)
		}
		if !strings.Contains(err.Error(), "boom") {
			t.Errorf("Error() = %q, want it to mention the panic value", err)
		}
	})

	t.Run("an error panic value is reachable with errors.Is", func(t *testing.T) {
		sentinel := errors.New("db down")
		err := funcs.Protect(func() error { panic(sentinel) })
		if !errors.Is(err, sentinel) {
			t.Errorf("err = %v, want errors.Is to find the panicked error", err)
		}
	})

	t.Run("panic(nil) is still reported as an error", func(t *testing.T) {
		err := funcs.Protect(func() error { panic(nil) })
		var pe *funcs.PanicError
		if !errors.As(err, &pe) {
			t.Errorf("err = %v (%T), want *funcs.PanicError for panic(nil)", err, err)
		}
	})
}

func TestPart3SafeGo(t *testing.T) {
	t.Run("a panic is reported from the goroutine", func(t *testing.T) {
		got := make(chan error, 1)
		funcs.SafeGo(func(err error) { got <- err }, func() { panic("kaboom") })
		select {
		case err := <-got:
			var pe *funcs.PanicError
			if !errors.As(err, &pe) {
				t.Fatalf("reported %v (%T), want *funcs.PanicError", err, err)
			}
			if pe.Value != any("kaboom") {
				t.Errorf("Value = %v, want \"kaboom\"", pe.Value)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("report was never called for a panicking goroutine")
		}
	})

	t.Run("a normal return runs fn and reports nothing", func(t *testing.T) {
		done := make(chan struct{})
		reported := make(chan error, 1)
		funcs.SafeGo(func(err error) { reported <- err }, func() { close(done) })
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("fn did not run")
		}
		select {
		case err := <-reported:
			t.Errorf("unexpected report: %v", err)
		case <-time.After(100 * time.Millisecond):
		}
	})
}
