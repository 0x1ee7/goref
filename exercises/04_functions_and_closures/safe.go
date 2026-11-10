package funcs

// PanicError is a recovered panic. Value is the value passed to panic and
// Stack is the goroutine stack captured at the point of recovery.
type PanicError struct {
	Value any
	Stack []byte
}

// Error returns "panic: <value>".
func (e *PanicError) Error() string { return "" }

// Unwrap returns Value when it is an error, and nil otherwise.
func (e *PanicError) Unwrap() error { return nil }

// Protect calls fn and returns its error unchanged. If fn panics, the panic is
// recovered and returned as a *PanicError.
func Protect(fn func() error) (err error) {
	return nil
}

// SafeGo runs fn in a new goroutine. If fn panics, the panic is recovered and
// report is called with a *PanicError from that goroutine; the process keeps
// running. If fn returns normally, report is not called. report must be non-nil.
func SafeGo(report func(error), fn func()) {}
