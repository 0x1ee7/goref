package funcs

import (
	"fmt"
	"runtime/debug"
)

// PanicError is the error form of a recovered panic. Stack holds the goroutine
// stack captured at the point of recovery.
type PanicError struct {
	Value any
	Stack []byte
}

// Error reports the panic value.
func (e *PanicError) Error() string {
	return fmt.Sprintf("panic: %v", e.Value)
}

// Unwrap returns the panic value when it is an error, so errors.Is and
// errors.As can see through a panic(err).
func (e *PanicError) Unwrap() error {
	err, _ := e.Value.(error)
	return err
}

// newPanicError captures the current stack. It must be called from the deferred
// function that recovered the panic so the stack still shows the panic site.
func newPanicError(v any) *PanicError {
	return &PanicError{Value: v, Stack: debug.Stack()}
}

// Protect calls fn and returns its error unchanged. If fn panics, Protect
// returns a *PanicError instead of unwinding the caller. panic(nil) also counts
// as a panic.
func Protect(fn func() error) (err error) {
	defer func() {
		if v := recover(); v != nil {
			err = newPanicError(v)
		}
	}()
	return fn()
}

// SafeGo runs fn in a new goroutine. A panic in fn is passed to report as a
// *PanicError instead of crashing the process. report is not called when fn
// returns normally.
func SafeGo(report func(error), fn func()) {
	go func() {
		defer func() {
			if v := recover(); v != nil {
				report(newPanicError(v))
			}
		}()
		fn()
	}()
}
