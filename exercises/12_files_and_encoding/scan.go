package backup

import (
	"errors"
	"io"
	"io/fs"
)

// ErrNotDirectory is returned by ScanDir when root exists but is not a directory.
var ErrNotDirectory = errors.New("not a directory")

// Scan walks fsys and returns the manifest of every regular file, keyed by
// slash-separated path relative to fsys's root. Symlinks and other non-regular
// entries are skipped, never followed. Files whose path matches one of the
// ignore patterns are left out (see the README for the matching rules).
//
// Each entry's Size is the number of bytes hashed, ModTime is the file's
// modification time in UTC truncated to microseconds, and Digest is the SHA-256
// of the content, streamed rather than read into memory.
//
// A malformed pattern is an error wrapping path.ErrBadPattern.
func Scan(fsys fs.FS, ignore ...string) (Manifest, error) {
	return nil, errors.New("not implemented")
}

// ScanDir is Scan for an operating-system directory. It opens root with
// os.OpenRoot so that nothing outside root can be reached, even through
// symlinks inside it. A missing root is an error satisfying errors.Is(err,
// fs.ErrNotExist); a root that is not a directory is ErrNotDirectory.
func ScanDir(root string, ignore ...string) (Manifest, error) {
	return nil, errors.New("not implemented")
}

// ParseIgnore reads an ignore file: one pattern per line. Surrounding
// whitespace is trimmed; blank lines and lines starting with '#' are skipped.
// A line longer than bufio's default token limit is an error wrapping
// bufio.ErrTooLong.
func ParseIgnore(r io.Reader) ([]string, error) {
	return nil, errors.New("not implemented")
}

// Hash streams r through SHA-256 and returns the digest and the number of
// bytes read. A read error is returned (wrapped or as-is) together with the
// zero Digest.
func Hash(r io.Reader) (Digest, int64, error) {
	return Digest{}, 0, errors.New("not implemented")
}
