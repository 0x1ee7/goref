package support

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
)

// envState is the value of one environment variable before WithEnv touched it.
type envState struct {
	value string
	ok    bool
}

// WithEnv applies changes to the process environment, runs fn, and restores
// the previous state afterwards. A nil value unsets the variable. Variables
// that did not exist before are removed again on restore.
//
// Restoration also runs when fn panics. If applying a change fails, the
// changes already applied are undone and the error is returned without
// calling fn. The process environment is global, so do not call WithEnv
// concurrently.
func WithEnv(changes map[string]*string, fn func() error) (err error) {
	keys := slices.Sorted(maps.Keys(changes))
	prev := make(map[string]envState, len(keys))
	var applied []string

	// Restore runs on every exit path. Only the keys that were actually
	// changed are restored, in reverse order.
	defer func() {
		var restoreErrs []error
		for _, k := range slices.Backward(applied) {
			p := prev[k]
			var rerr error
			if p.ok {
				rerr = os.Setenv(k, p.value)
			} else {
				rerr = os.Unsetenv(k)
			}
			if rerr != nil {
				restoreErrs = append(restoreErrs, fmt.Errorf("restore %s: %w", k, rerr))
			}
		}
		if rerr := errors.Join(restoreErrs...); rerr != nil {
			err = errors.Join(err, rerr)
		}
	}()

	for _, k := range keys {
		value, ok := os.LookupEnv(k)
		prev[k] = envState{value: value, ok: ok}

		var aerr error
		if v := changes[k]; v == nil {
			aerr = os.Unsetenv(k)
		} else {
			aerr = os.Setenv(k, *v)
		}
		if aerr != nil {
			return fmt.Errorf("set %s: %w", k, aerr)
		}
		applied = append(applied, k)
	}
	return fn()
}

// OpenAll opens paths in order with open and returns the readers in the same
// order.
//
// If any open fails, OpenAll closes the readers it already opened, in reverse
// order, and returns a nil slice. The error wraps the open failure with its
// path. Close failures are joined to it, so both remain matchable with
// errors.Is.
func OpenAll(paths []string, open func(name string) (io.ReadCloser, error)) ([]io.ReadCloser, error) {
	rcs := make([]io.ReadCloser, 0, len(paths))
	for _, p := range paths {
		rc, err := open(p)
		if err == nil {
			rcs = append(rcs, rc)
			continue
		}
		errs := []error{fmt.Errorf("open %s: %w", p, err)}
		for _, opened := range slices.Backward(rcs) {
			if cerr := opened.Close(); cerr != nil {
				errs = append(errs, cerr)
			}
		}
		return nil, errors.Join(errs...)
	}
	return rcs, nil
}

// RemoveFiles deletes each path in order and returns how many were removed.
// A path that does not exist is skipped. Any other failure stops the loop and
// is returned with the count so far.
func RemoveFiles(paths []string) (int, error) {
	n := 0
	for _, p := range paths {
		err := os.Remove(p)
		switch {
		case err == nil:
			n++
		case errors.Is(err, os.ErrNotExist):
			// Already gone; nothing to do.
		default:
			return n, fmt.Errorf("remove %s: %w", p, err)
		}
	}
	return n, nil
}

// WriteFileAtomic replaces the file at path with data.
//
// It writes to a temporary file in the same directory, syncs it, closes it,
// and renames it over path. Readers therefore see either the old or the new
// content, never a partial write. On any failure the temporary file is removed
// and path is unchanged. Errors from Close are reported.
func WriteFileAtomic(path string, data []byte) (err error) {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	closed := false
	defer func() {
		if err != nil {
			if !closed {
				_ = f.Close()
			}
			_ = os.Remove(tmp)
		}
	}()

	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	closed = true
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
