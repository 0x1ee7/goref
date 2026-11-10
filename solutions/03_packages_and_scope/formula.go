package scope

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"maps"
	"math"
	"slices"
	"strconv"
)

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
	return map[string]Func{
		"ABS":   unary("ABS", math.Abs),
		"ROUND": unary("ROUND", math.Round),
		"MIN": func(args ...float64) (float64, error) {
			if len(args) == 0 {
				return 0, errors.New("MIN needs at least one argument")
			}
			return slices.Min(args), nil
		},
		"MAX": func(args ...float64) (float64, error) {
			if len(args) == 0 {
				return 0, errors.New("MAX needs at least one argument")
			}
			return slices.Max(args), nil
		},
	}
}

func unary(name string, f func(float64) float64) Func {
	return func(args ...float64) (float64, error) {
		if len(args) != 1 {
			return 0, fmt.Errorf("%s takes 1 argument, got %d", name, len(args))
		}
		return f(args[0]), nil
	}
}

// Evaluate parses expr with go/parser and evaluates it using only whitelisted
// syntax. Names resolve in cells first and must not clash with functions.
// funcs are merged over DefaultFunctions. Both maps may be nil.
func Evaluate(expr string, cells map[string]float64, funcs map[string]Func) (float64, error) {
	if err := rejectComments(expr); err != nil {
		return 0, err
	}
	node, err := parser.ParseExpr(expr)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrParse, err)
	}
	merged := DefaultFunctions()
	maps.Copy(merged, funcs)
	for name := range cells {
		if _, ok := merged[name]; ok {
			return 0, fmt.Errorf("%w: %q", ErrConflict, name)
		}
	}
	ev := &evaluator{cells: cells, funcs: merged}
	v, err := ev.eval(node)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("%w: result %v is not finite", ErrEval, v)
	}
	return v, nil
}

// rejectComments fails if expr contains a // or /* */ comment. go/parser
// discards comments silently, so "1 // 2" would otherwise evaluate to 1.
func rejectComments(expr string) error {
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(expr))
	var s scanner.Scanner
	s.Init(file, []byte(expr), nil, scanner.ScanComments)
	for {
		_, tok, _ := s.Scan()
		switch tok {
		case token.EOF:
			return nil
		case token.COMMENT:
			return fmt.Errorf("%w: comments are not allowed", ErrForbidden)
		}
	}
}

type evaluator struct {
	cells map[string]float64
	funcs map[string]Func
}

func (ev *evaluator) eval(n ast.Expr) (float64, error) {
	switch n := n.(type) {
	case *ast.ParenExpr:
		return ev.eval(n.X)
	case *ast.BasicLit:
		return literal(n)
	case *ast.Ident:
		v, ok := ev.cells[n.Name]
		if !ok {
			return 0, fmt.Errorf("%w: %q", ErrUnknownName, n.Name)
		}
		return v, nil
	case *ast.UnaryExpr:
		if n.Op != token.ADD && n.Op != token.SUB {
			return 0, fmt.Errorf("%w: unary %s", ErrForbidden, n.Op)
		}
		v, err := ev.eval(n.X)
		if err != nil {
			return 0, err
		}
		if n.Op == token.SUB {
			return -v, nil
		}
		return v, nil
	case *ast.BinaryExpr:
		switch n.Op {
		case token.ADD, token.SUB, token.MUL, token.QUO, token.REM:
		default:
			return 0, fmt.Errorf("%w: operator %s", ErrForbidden, n.Op)
		}
		l, err := ev.eval(n.X)
		if err != nil {
			return 0, err
		}
		r, err := ev.eval(n.Y)
		if err != nil {
			return 0, err
		}
		return apply(n.Op, l, r)
	case *ast.CallExpr:
		return ev.call(n)
	default:
		return 0, fmt.Errorf("%w: %T", ErrForbidden, n)
	}
}

func (ev *evaluator) call(c *ast.CallExpr) (float64, error) {
	if c.Ellipsis.IsValid() {
		return 0, fmt.Errorf("%w: variadic spread", ErrForbidden)
	}
	id, ok := c.Fun.(*ast.Ident)
	if !ok {
		return 0, fmt.Errorf("%w: call target must be a plain name", ErrForbidden)
	}
	fn, ok := ev.funcs[id.Name]
	if !ok || fn == nil {
		return 0, fmt.Errorf("%w: %q", ErrUnknownName, id.Name)
	}
	args := make([]float64, 0, len(c.Args))
	for _, a := range c.Args {
		v, err := ev.eval(a)
		if err != nil {
			return 0, err
		}
		args = append(args, v)
	}
	v, err := fn(args...)
	if err != nil {
		return 0, fmt.Errorf("%w: %s: %w", ErrEval, id.Name, err)
	}
	return v, nil
}

func literal(lit *ast.BasicLit) (float64, error) {
	switch lit.Kind {
	case token.INT:
		i, err := strconv.ParseInt(lit.Value, 0, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: integer %s: %v", ErrParse, lit.Value, err)
		}
		return float64(i), nil
	case token.FLOAT:
		f, err := strconv.ParseFloat(lit.Value, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: float %s: %v", ErrParse, lit.Value, err)
		}
		return f, nil
	default:
		return 0, fmt.Errorf("%w: %s literal", ErrForbidden, lit.Kind)
	}
}

func apply(op token.Token, l, r float64) (float64, error) {
	switch op {
	case token.ADD:
		return l + r, nil
	case token.SUB:
		return l - r, nil
	case token.MUL:
		return l * r, nil
	case token.QUO:
		if r == 0 {
			return 0, fmt.Errorf("%w: division by zero", ErrEval)
		}
		return l / r, nil
	default: // token.REM
		if r == 0 {
			return 0, fmt.Errorf("%w: modulo by zero", ErrEval)
		}
		return math.Mod(l, r), nil
	}
}
