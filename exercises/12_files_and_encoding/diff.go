package backup

import (
	"errors"
	"io/fs"
	"os"
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
	return false
}

// Diff compares old with new. Added are keys only in new, Removed keys only in
// old, Modified keys in both whose Size or Digest differ. A ModTime-only change
// is not a modification.
func Diff(old, new Manifest) ManifestDiff {
	return ManifestDiff{}
}

// CheckRelPath reports whether name is a safe manifest key. It returns an
// error wrapping ErrUnsafePath for names that are empty, absolute, contain
// "." or ".." or empty segments, contain a backslash or a colon, or otherwise
// fail fs.ValidPath. Names such as "..foo" are valid.
func CheckRelPath(name string) error {
	return errors.New("not implemented")
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
	return ApplyResult{}, errors.New("not implemented")
}
