package backup

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"
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
	pats, err := compileIgnore(ignore)
	if err != nil {
		return nil, err
	}
	m := Manifest{}
	err = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." {
			return nil
		}
		if pats.matchesComponent(d.Name()) && d.IsDir() {
			return fs.SkipDir // a bare pattern naming a directory prunes its subtree
		}
		if !d.Type().IsRegular() || pats.matches(p) {
			return nil
		}
		e, err := entryFor(fsys, p)
		if err != nil {
			return err
		}
		m[p] = e
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	return m, nil
}

// ScanDir is Scan for an operating-system directory. It opens root with
// os.OpenRoot so that nothing outside root can be reached, even through
// symlinks inside it. A missing root is an error satisfying errors.Is(err,
// fs.ErrNotExist); a root that is not a directory is ErrNotDirectory.
func ScanDir(root string, ignore ...string) (Manifest, error) {
	fi, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("scan dir: %w", err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("scan dir %s: %w", root, ErrNotDirectory)
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("scan dir: %w", err)
	}
	defer r.Close()
	return Scan(r.FS(), ignore...)
}

// ParseIgnore reads an ignore file: one pattern per line. Surrounding
// whitespace is trimmed; blank lines and lines starting with '#' are skipped.
// A line longer than bufio's default token limit is an error wrapping
// bufio.ErrTooLong.
func ParseIgnore(r io.Reader) ([]string, error) {
	sc := bufio.NewScanner(r)
	var out []string
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("parse ignore: %w", err)
	}
	return out, nil
}

// Hash streams r through SHA-256 and returns the digest and the number of
// bytes read. A read error is returned (wrapped or as-is) together with the
// zero Digest.
func Hash(r io.Reader) (Digest, int64, error) {
	h := sha256.New()
	n, err := io.Copy(h, r)
	if err != nil {
		return Digest{}, n, err
	}
	var d Digest
	copy(d[:], h.Sum(nil))
	return d, n, nil
}

// entryFor hashes one regular file and builds its manifest entry.
func entryFor(fsys fs.FS, p string) (Entry, error) {
	f, err := fsys.Open(p)
	if err != nil {
		return Entry{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return Entry{}, err
	}
	digest, n, err := Hash(f)
	if err != nil {
		return Entry{}, fmt.Errorf("hash %s: %w", p, err)
	}
	return Entry{
		Size:    n,
		ModTime: info.ModTime().UTC().Truncate(time.Microsecond),
		Digest:  digest,
	}, nil
}

// ignoreSet is a validated list of ignore patterns, split for matching.
type ignoreSet struct {
	bare  []string   // no slash: matches any path component
	paths [][]string // with slash: matched segment-wise against the whole path
}

func compileIgnore(patterns []string) (ignoreSet, error) {
	var s ignoreSet
	for _, pat := range patterns {
		if pat == "" {
			return ignoreSet{}, fmt.Errorf("scan: empty ignore pattern: %w", path.ErrBadPattern)
		}
		segs := strings.Split(pat, "/")
		for _, seg := range segs {
			if seg == "**" {
				continue
			}
			if _, err := path.Match(seg, ""); err != nil {
				return ignoreSet{}, fmt.Errorf("scan: ignore pattern %q: %w", pat, err)
			}
		}
		if len(segs) == 1 {
			s.bare = append(s.bare, pat)
		} else {
			s.paths = append(s.paths, segs)
		}
	}
	return s, nil
}

// matchesComponent reports whether a bare pattern matches a single name.
func (s ignoreSet) matchesComponent(name string) bool {
	for _, pat := range s.bare {
		if ok, _ := path.Match(pat, name); ok {
			return true
		}
	}
	return false
}

// matches reports whether the relative file path p is ignored.
func (s ignoreSet) matches(p string) bool {
	for _, c := range strings.Split(p, "/") {
		if s.matchesComponent(c) {
			return true
		}
	}
	segs := strings.Split(p, "/")
	for _, pat := range s.paths {
		if matchSegments(pat, segs) {
			return true
		}
	}
	return false
}

// matchSegments matches pat against name segment by segment. A "**" segment
// matches zero or more whole segments; any other segment uses path.Match, so a
// single "*" never crosses a slash.
func matchSegments(pat, name []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			for i := 0; i <= len(name); i++ {
				if matchSegments(pat[1:], name[i:]) {
					return true
				}
			}
			return false
		}
		if len(name) == 0 {
			return false
		}
		if ok, _ := path.Match(pat[0], name[0]); !ok {
			return false
		}
		pat, name = pat[1:], name[1:]
	}
	return len(name) == 0
}
