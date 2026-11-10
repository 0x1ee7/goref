package support

import "io"

// WithEnv applies changes to the process environment, runs fn, and restores
// the previous state afterwards. A nil value unsets the variable. Variables
// that did not exist before are removed again on restore.
//
// Restoration also runs when fn panics. If applying a change fails, the
// changes already applied are undone and the error is returned without
// calling fn. The process environment is global, so do not call WithEnv
// concurrently.
func WithEnv(changes map[string]*string, fn func() error) error { return nil }

// OpenAll opens paths in order with open and returns the readers in the same
// order.
//
// If any open fails, OpenAll closes the readers it already opened, in reverse
// order, and returns a nil slice. The error wraps the open failure with its
// path. Close failures are joined to it, so both remain matchable with
// errors.Is.
func OpenAll(paths []string, open func(name string) (io.ReadCloser, error)) ([]io.ReadCloser, error) {
	return nil, nil
}

// RemoveFiles deletes each path in order and returns how many were removed.
// A path that does not exist is skipped. Any other failure stops the loop and
// is returned with the count so far.
func RemoveFiles(paths []string) (int, error) { return 0, nil }

// WriteFileAtomic replaces the file at path with data.
//
// It writes to a temporary file in the same directory, syncs it, closes it,
// and renames it over path. Readers therefore see either the old or the new
// content, never a partial write. On any failure the temporary file is removed
// and path is unchanged. Errors from Close are reported.
func WriteFileAtomic(path string, data []byte) error { return nil }
