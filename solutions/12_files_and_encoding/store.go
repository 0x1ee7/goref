package backup

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"time"
)

// WriteFileAtomic replaces name with data. It writes a temporary file in the
// same directory, flushes it to stable storage, sets perm, and renames it over
// name. On any failure the temporary file is removed and the error returned;
// an existing name is left untouched. A missing parent directory yields an
// error satisfying errors.Is(err, fs.ErrNotExist).
func WriteFileAtomic(name string, data []byte, perm fs.FileMode) (err error) {
	f, err := os.CreateTemp(filepath.Dir(name), "."+filepath.Base(name)+".tmp-*")
	if err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
		}
	}()
	if _, err = f.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	if err = f.Sync(); err != nil {
		return fmt.Errorf("sync %s: %w", name, err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", name, err)
	}
	if err = os.Chmod(tmp, perm); err != nil {
		return fmt.Errorf("chmod %s: %w", name, err)
	}
	if err = os.Rename(tmp, name); err != nil {
		return fmt.Errorf("rename %s: %w", name, err)
	}
	return nil
}

// Document is the persisted form of a manifest.
type Document struct {
	Created time.Time `json:"created,omitzero"`
	Entries Manifest  `json:"entries"`
}

// SaveManifest encodes doc as indented JSON with a leading "format": 1 field
// and persists it with WriteFileAtomic. Nothing is written if encoding fails.
func SaveManifest(name string, doc Document) error {
	body := struct {
		Format int `json:"format"`
		Document
	}{Format: 1, Document: doc}
	b, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	return WriteFileAtomic(name, append(b, '\n'), 0o644)
}

// LoadManifest reads and strictly decodes a document written by SaveManifest.
// Unknown fields, a format other than 1, malformed digests and trailing data
// after the JSON value are errors.
func LoadManifest(name string) (Document, error) {
	f, err := os.Open(name)
	if err != nil {
		return Document{}, fmt.Errorf("load manifest: %w", err)
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var raw struct {
		Format int `json:"format"`
		Document
	}
	if err := dec.Decode(&raw); err != nil {
		return Document{}, fmt.Errorf("load manifest %s: %w", name, err)
	}
	if raw.Format != 1 {
		return Document{}, fmt.Errorf("load manifest %s: unsupported format %d", name, raw.Format)
	}
	if _, err := dec.Token(); err != io.EOF {
		return Document{}, fmt.Errorf("load manifest %s: trailing data after document", name)
	}
	return raw.Document, nil
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
	return func(yield func(Record, error) bool) {
		fail := func(err error) {
			yield(Record{}, fmt.Errorf("stream records: %w", err))
		}
		dec := json.NewDecoder(r)
		dec.DisallowUnknownFields()

		tok, err := dec.Token()
		if err != nil {
			fail(err)
			return
		}
		if d, ok := tok.(json.Delim); !ok || d != '[' {
			fail(errors.New("want a JSON array"))
			return
		}
		for dec.More() {
			var rec Record
			if err := dec.Decode(&rec); err != nil {
				fail(err)
				return
			}
			if !yield(rec, nil) {
				return
			}
		}
		if tok, err = dec.Token(); err != nil {
			fail(err)
			return
		}
		if d, ok := tok.(json.Delim); !ok || d != ']' {
			fail(errors.New("unterminated array"))
			return
		}
		if _, err := dec.Token(); err != io.EOF {
			fail(errors.New("trailing data after array"))
		}
	}
}
