package functional

// Compose returns f∘g: the function x ↦ f(g(x)). Composition is right-to-left,
// like mathematical notation; B flows from g into f.
func Compose[A, B, C any](f func(B) C, g func(A) B) func(A) C {
	return func(a A) C { return f(g(a)) }
}

// Pipe threads v through stages from left to right and returns the result. The
// stages share one type, so the chain is just a loop. With no stages it returns
// v unchanged.
func Pipe[T any](v T, stages ...func(T) T) T {
	for _, stage := range stages {
		v = stage(v)
	}
	return v
}

// Curry2 turns a two-argument function into a chain of one-argument calls. Each
// partial application captures its own argument, so applying one result never
// affects another.
func Curry2[A, B, R any](f func(A, B) R) func(A) func(B) R {
	return func(a A) func(B) R {
		return func(b B) R { return f(a, b) }
	}
}

// Curry3 is Curry2 for three-argument functions.
func Curry3[A, B, C, R any](f func(A, B, C) R) func(A) func(B) func(C) R {
	return func(a A) func(B) func(C) R {
		return func(b B) func(C) R {
			return func(c C) R { return f(a, b, c) }
		}
	}
}

// Flip returns f with its first two arguments swapped.
func Flip[A, B, R any](f func(A, B) R) func(B, A) R {
	return func(b B, a A) R { return f(a, b) }
}

// Bind fixes the first argument of f, the closure analogue of
// functools.partial. Go has no default or keyword arguments, so binding the
// leading argument is the practical substitute.
func Bind[A, B, R any](f func(A, B) R, a A) func(B) R {
	return func(b B) R { return f(a, b) }
}
