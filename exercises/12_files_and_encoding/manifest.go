// Package backup is the core of a small backup/sync tool: scan a tree into a
// manifest, diff manifests, apply diffs safely, persist state atomically, and
// write CSV reports, gzip streams and a binary snapshot header.
//
// Everything is built on the standard library: io/fs for abstraction, os.Root
// for traversal-safe access, encoding/json, encoding/csv, compress/gzip,
// encoding/binary and bufio.
package backup

import (
	"crypto/sha256"
	"errors"
	"iter"
	"time"
)

// Digest is a SHA-256 digest.
type Digest [sha256.Size]byte

// ParseDigest parses a 64-character hexadecimal SHA-256 (either case).
func ParseDigest(s string) (Digest, error) {
	return Digest{}, errors.New("not implemented")
}

// String returns the 64-character lowercase hexadecimal form.
func (d Digest) String() string {
	return ""
}

// MarshalJSON encodes d as a JSON string of lowercase hexadecimal.
func (d Digest) MarshalJSON() ([]byte, error) {
	return nil, errors.New("not implemented")
}

// UnmarshalJSON decodes a JSON string holding exactly 64 hexadecimal characters.
func (d *Digest) UnmarshalJSON(b []byte) error {
	return errors.New("not implemented")
}

// Entry is the manifest record for one regular file.
type Entry struct {
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mtime"`
	Digest  Digest    `json:"sha256"`
}

// Manifest maps slash-separated relative paths to entries.
type Manifest map[string]Entry

// Paths yields the manifest keys in sorted order.
func (m Manifest) Paths() iter.Seq[string] {
	return func(yield func(string) bool) {}
}
