package funcs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Cleanup collects teardown functions and runs them last-in, first-out. The
// zero value is ready to use. Run executes every registered function even when
// some fail, and reports all failures joined in run order. Each function runs
// at most once: a second Run is a no-op.
type Cleanup struct {
	fns []func() error
}

// Defer registers fn to run during Run.
func (c *Cleanup) Defer(fn func() error) {
	c.fns = append(c.fns, fn)
}

// Run executes the registered functions in reverse registration order and
// returns their errors joined, or nil if all succeeded.
func (c *Cleanup) Run() error {
	fns := c.fns
	c.fns = nil
	var errs []error
	for i := len(fns) - 1; i >= 0; i-- {
		if err := fns[i](); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Annotate wraps a non-nil *err with a context message, as "<format>: <err>".
// It is meant for use in a defer on a function with a named error result. The
// arguments are evaluated when the defer statement runs. A nil error is left
// untouched.
func Annotate(err *error, format string, args ...any) {
	if *err == nil {
		return
	}
	*err = fmt.Errorf(format+": %w", append(args, *err)...)
}

// WriteFileAtomic writes data to path so readers see either the old contents
// or the new ones, never a partial write. It writes a temporary file in the
// same directory, syncs it, closes it with the error checked, and renames it
// over path. On failure the temporary file is removed. Errors are prefixed
// with "write <path>: ".
func WriteFileAtomic(path string, data []byte) (err error) {
	defer Annotate(&err, "write %s", path)

	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	committed := false
	defer func() {
		if !committed {
			_ = f.Close() // already closed on the success path; error is irrelevant here
			_ = os.Remove(tmp)
		}
	}()

	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	committed = true
	return nil
}

// ForEachFile opens each path in order and passes it to fn, closing the file
// before moving on. A failure on one path does not stop the others. Open,
// callback, and close errors are wrapped with the path and joined.
func ForEachFile(paths []string, fn func(path string, f *os.File) error) error {
	var errs []error
	for _, p := range paths {
		if err := forEachOne(p, fn); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func forEachOne(path string, fn func(path string, f *os.File) error) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	if err := fn(path, f); err != nil {
		err = fmt.Errorf("%s: %w", path, err)
		return errors.Join(err, closeFile(path, f))
	}
	return closeFile(path, f)
}

func closeFile(path string, f *os.File) error {
	if err := f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}
	return nil
}
