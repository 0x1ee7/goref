package backup

import (
	"errors"
	"io"
	"time"
)

// ReportRow is one data row of a CSV report.
type ReportRow struct {
	Path    string
	Size    int64
	ModTime time.Time
	Digest  Digest
	Note    string
}

// WriteReport writes m as a CSV report with the header
// path,size,mtime,sha256,note followed by one row per entry, sorted by path.
// size is decimal, mtime is RFC 3339 with nanoseconds in UTC, and note is
// notes[path]. Every key in notes must exist in m; otherwise an error is
// returned and nothing is written.
func WriteReport(w io.Writer, m Manifest, notes map[string]string) error {
	return errors.New("not implemented")
}

// ReadReport parses a report written by WriteReport. A leading UTF-8 byte
// order mark and CRLF line endings are accepted. The header must match exactly
// and every row must have five fields; errors name the offending line.
func ReadReport(r io.Reader) ([]ReportRow, error) {
	return nil, errors.New("not implemented")
}
