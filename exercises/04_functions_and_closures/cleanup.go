package funcs

import "os"

// Cleanup collects deferred cleanup functions and runs them in LIFO order.
// The zero value is ready to use.
type Cleanup struct{}

// Defer registers fn to be run by Run.
func (c *Cleanup) Defer(fn func() error) {}

// Run executes the registered functions in LIFO order. Every function runs
// even if earlier ones fail; the errors are joined with errors.Join in the
// order they occurred. A second Run runs nothing and returns nil.
func (c *Cleanup) Run() error { return nil }

// Annotate is meant to be deferred with a named error result. If *err is
// non-nil it is replaced by fmt.Errorf(format+": %w", args..., *err).
// A nil *err is left untouched.
func Annotate(err *error, format string, args ...any) {}

// WriteFileAtomic replaces path with data: it writes a temporary file in the
// same directory, syncs and closes it (checking the Close error), then renames
// it over path. Any failure is returned prefixed with "write <path>: ", and
// no temporary file is left behind.
func WriteFileAtomic(path string, data []byte) (err error) {
	return nil
}

// ForEachFile opens each path in order, calls fn with the open file and closes
// it before moving on. Failures do not stop the loop: open, fn and close errors
// are wrapped with the path and all are returned joined (nil if none).
func ForEachFile(paths []string, fn func(path string, f *os.File) error) error {
	return nil
}
