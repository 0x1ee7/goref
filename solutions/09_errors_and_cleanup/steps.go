package support

import "fmt"

// StepError reports that a named step failed. Err is the step's error and is
// reachable through Unwrap.
type StepError struct {
	Step string
	Err  error
}

// Error renders the step name and cause.
func (e *StepError) Error() string { return fmt.Sprintf("step %s: %v", e.Step, e.Err) }

// Unwrap returns the step's error.
func (e *StepError) Unwrap() error { return e.Err }

// RunStep runs step and reports its progress through trace.
//
// trace receives "start <name>" before the step runs and "end <name>" when
// RunStep returns, including when step panics. It receives "ok <name>" on
// success and "failed <name>" when step returns an error. On failure the
// error is returned as a *StepError and the value is the zero T. A panic from
// step is not converted; it propagates to the caller.
func RunStep[T any](name string, trace func(event string), step func() (T, error)) (T, error) {
	trace("start " + name)
	defer trace("end " + name)

	v, err := step()
	if err != nil {
		trace("failed " + name)
		var zero T
		return zero, &StepError{Step: name, Err: err}
	}
	trace("ok " + name)
	return v, nil
}
