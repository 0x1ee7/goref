package backup

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
)

// ErrUnsafePath is wrapped by errors for names that are not safe relative
// manifest keys.
var ErrUnsafePath = errors.New("unsafe path")

// ManifestDiff describes how a destination must change to match a source.
// Every slice is sorted.
type ManifestDiff struct {
	Added    []string
	Removed  []string
	Modified []string
}

// Empty reports whether the diff contains no changes.
func (d ManifestDiff) Empty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Modified) == 0
}

// Diff compares old with new. Added are keys only in new, Removed keys only in
// old, Modified keys in both whose Size or Digest differ. A ModTime-only change
// is not a modification.
func Diff(old, new Manifest) ManifestDiff {
	var d ManifestDiff
	for p, ne := range new {
		oe, ok := old[p]
		switch {
		case !ok:
			d.Added = append(d.Added, p)
		case oe.Size != ne.Size || oe.Digest != ne.Digest:
			d.Modified = append(d.Modified, p)
		}
	}
	for p := range old {
		if _, ok := new[p]; !ok {
			d.Removed = append(d.Removed, p)
		}
	}
	slices.Sort(d.Added)
	slices.Sort(d.Removed)
	slices.Sort(d.Modified)
	return d
}

// CheckRelPath reports whether name is a safe manifest key. It returns an
// error wrapping ErrUnsafePath for names that are empty, absolute, contain
// "." or ".." or empty segments, contain a backslash or a colon, or otherwise
// fail fs.ValidPath. Names such as "..foo" are valid.
func CheckRelPath(name string) error {
	if strings.ContainsAny(name, `\:`) {
		return fmt.Errorf("%w: %q contains a backslash or colon", ErrUnsafePath, name)
	}
	if name == "." || !fs.ValidPath(name) {
		return fmt.Errorf("%w: %q", ErrUnsafePath, name)
	}
	return nil
}

// ApplyResult reports what Apply changed. Both slices are sorted.
type ApplyResult struct {
	Copied  []string // added and modified paths copied from the source
	Deleted []string // removed paths actually deleted from the destination
}

// Apply makes dst match d, reading new content from src. Every path in d is
// checked with CheckRelPath before anything is touched. Added and modified
// files are copied (parents created, mtime preserved from src); removed files
// are deleted, and a removed file that is already absent is not an error and is
// not reported. Parent directories left empty by a deletion are removed,
// climbing up to but not including the root. All access goes through dst, so
// writes cannot escape it.
func Apply(d ManifestDiff, src fs.FS, dst *os.Root) (ApplyResult, error) {
	for _, group := range [][]string{d.Added, d.Modified, d.Removed} {
		for _, p := range group {
			if err := CheckRelPath(p); err != nil {
				return ApplyResult{}, fmt.Errorf("apply: %w", err)
			}
		}
	}

	var res ApplyResult
	for _, p := range slices.Concat(d.Added, d.Modified) {
		if err := copyFile(src, dst, p); err != nil {
			return res, fmt.Errorf("apply: copy %s: %w", p, err)
		}
		res.Copied = append(res.Copied, p)
	}
	slices.Sort(res.Copied)

	for _, p := range d.Removed {
		err := dst.Remove(p)
		switch {
		case err == nil:
			res.Deleted = append(res.Deleted, p)
			removeEmptyParents(dst, p)
		case errors.Is(err, fs.ErrNotExist):
			// Already gone: nothing to do and nothing to report.
		default:
			return res, fmt.Errorf("apply: remove %s: %w", p, err)
		}
	}
	slices.Sort(res.Deleted)
	return res, nil
}

func copyFile(src fs.FS, dst *os.Root, name string) error {
	in, err := src.Open(name)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := mkdirParents(dst, path.Dir(name)); err != nil {
		return err
	}
	out, err := dst.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return dst.Chtimes(name, info.ModTime(), info.ModTime())
}

// mkdirParents creates dir and any missing ancestors inside root.
func mkdirParents(dst *os.Root, dir string) error {
	if dir == "." {
		return nil
	}
	cur := ""
	for seg := range strings.SplitSeq(dir, "/") {
		cur = path.Join(cur, seg)
		if err := dst.Mkdir(cur, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
	}
	return nil
}

// removeEmptyParents deletes ancestors of name that are now empty, stopping at
// the first directory that cannot be removed.
func removeEmptyParents(dst *os.Root, name string) {
	for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
		if err := dst.Remove(dir); err != nil {
			return
		}
	}
}
