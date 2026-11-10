package backup

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
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

var reportHeader = []string{"path", "size", "mtime", "sha256", "note"}

// WriteReport writes m as a CSV report with the header
// path,size,mtime,sha256,note followed by one row per entry, sorted by path.
// size is decimal, mtime is RFC 3339 with nanoseconds in UTC, and note is
// notes[path]. Every key in notes must exist in m; otherwise an error is
// returned and nothing is written.
func WriteReport(w io.Writer, m Manifest, notes map[string]string) error {
	for p := range maps.Keys(notes) {
		if _, ok := m[p]; !ok {
			return fmt.Errorf("write report: note for %q: no such entry", p)
		}
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(reportHeader); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	for p := range m.Paths() {
		e := m[p]
		row := []string{
			p,
			strconv.FormatInt(e.Size, 10),
			e.ModTime.UTC().Format(time.RFC3339Nano),
			e.Digest.String(),
			notes[p],
		}
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("write report: %w", err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

// ReadReport parses a report written by WriteReport. A leading UTF-8 byte
// order mark and CRLF line endings are accepted. The header must match exactly
// and every row must have five fields; errors name the offending line.
func ReadReport(r io.Reader) ([]ReportRow, error) {
	br := bufio.NewReader(r)
	if bom, err := br.Peek(3); err == nil && bytes.Equal(bom, []byte("\xEF\xBB\xBF")) {
		_, _ = br.Discard(3)
	}

	cr := csv.NewReader(br)
	cr.FieldsPerRecord = len(reportHeader)
	header, err := cr.Read()
	if errors.Is(err, io.EOF) {
		return nil, errors.New("read report: missing header")
	}
	if err != nil {
		return nil, fmt.Errorf("read report: header: %w", err)
	}
	if !slices.Equal(header, reportHeader) {
		return nil, fmt.Errorf("read report: header %q, want %q", header, reportHeader)
	}

	var rows []ReportRow
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return rows, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read report: %w", err)
		}
		line, _ := cr.FieldPos(0)
		row, err := parseReportRow(rec)
		if err != nil {
			return nil, fmt.Errorf("read report: line %d: %w", line, err)
		}
		rows = append(rows, row)
	}
}

func parseReportRow(rec []string) (ReportRow, error) {
	size, err := strconv.ParseInt(rec[1], 10, 64)
	if err != nil || size < 0 {
		return ReportRow{}, fmt.Errorf("size %q: not a non-negative integer", rec[1])
	}
	mtime, err := time.Parse(time.RFC3339Nano, rec[2])
	if err != nil {
		return ReportRow{}, fmt.Errorf("mtime %q: %w", rec[2], err)
	}
	digest, err := ParseDigest(rec[3])
	if err != nil {
		return ReportRow{}, err
	}
	return ReportRow{Path: rec[0], Size: size, ModTime: mtime, Digest: digest, Note: rec[4]}, nil
}
