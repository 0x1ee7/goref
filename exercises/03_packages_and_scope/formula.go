package scope

import "errors"

var (
	// ErrParse reports input that go/parser rejects, including empty input and
	// literals that do not fit their type.
	ErrParse = errors.New("formula syntax error")
	// ErrForbidden reports valid Go syntax outside the formula grammar,
	// including comments.
	ErrForbidden = errors.New("formula syntax not allowed")
	// ErrUnknownName reports a value name that is not a cell, or a call target that is not a function.
	ErrUnknownName = errors.New("unknown name")
	// ErrConflict reports a name that is both a cell and a function.
	ErrConflict = errors.New("name is both a cell and a function")
	// ErrEval reports a runtime failure: division by zero, a function error,
	// bad arity, or a non-finite result.
	ErrEval = errors.New("formula evaluation failed")
)

// Func is a function callable from a formula. Arguments are already evaluated
// to float64.
type Func func(args ...float64) (float64, error)

// DefaultFunctions returns a fresh map with the built-in functions ABS, MIN,
// MAX and ROUND. Each call returns a new map the caller may modify freely.
func DefaultFunctions() map[string]Func {
	return nil
}

// Evaluate parses expr with go/parser and evaluates it using only whitelisted
// syntax. Names resolve in cells first and must not clash with functions.
// funcs are merged over DefaultFunctions. Both maps may be nil.
func Evaluate(expr string, cells map[string]float64, funcs map[string]Func) (float64, error) {
	return 0, errors.New("not implemented")
}
