package backup

import (
	"errors"
	"io"
	"io/fs"
	"iter"
	"time"
)

// Document is the persisted form of a manifest.
type Document struct {
	Created time.Time `json:"created,omitzero"`
	Entries Manifest  `json:"entries"`
}

// WriteFileAtomic replaces name with data. It writes a temporary file in the
// same directory, flushes it to stable storage, sets perm, and renames it over
// name. On any failure the temporary file is removed and the error returned;
// an existing name is left untouched. A missing parent directory yields an
// error satisfying errors.Is(err, fs.ErrNotExist).
func WriteFileAtomic(name string, data []byte, perm fs.FileMode) error {
	return errors.New("not implemented")
}

// SaveManifest encodes doc as indented JSON with a leading "format": 1 field
// and persists it with WriteFileAtomic. Nothing is written if encoding fails.
func SaveManifest(name string, doc Document) error {
	return errors.New("not implemented")
}

// LoadManifest reads and strictly decodes a document written by SaveManifest.
// Unknown fields, a format other than 1, malformed digests and trailing data
// after the JSON value are errors.
func LoadManifest(name string) (Document, error) {
	return Document{}, errors.New("not implemented")
}

// Record is one line of an exported manifest stream. Its Entry is embedded, so
// the JSON object is flat: {"path":..., "size":..., "mtime":..., "sha256":...}.
type Record struct {
	Path string `json:"path"`
	Entry
}

// StreamRecords decodes a JSON array of Records from r without loading it into
// memory. It yields each record as it is decoded. Malformed input (including a
// top-level value that is not an array, unknown fields, empty input, or data
// after the closing bracket) yields one (Record{}, err) and stops. If the
// consumer stops early, decoding stops and no error is yielded.
func StreamRecords(r io.Reader) iter.Seq2[Record, error] {
	return func(yield func(Record, error) bool) {}
}
