package interfaces

import (
	"errors"
	"io"
)

// ErrLimitExceeded is returned by LimitWriter once its budget is spent.
var ErrLimitExceeded = errors.New("write limit exceeded")

// CountingWriter counts the bytes its underlying writer accepted (Part 4).
type CountingWriter struct{}

// NewCountingWriter wraps w.
func NewCountingWriter(w io.Writer) *CountingWriter { return &CountingWriter{} }

// Write forwards p and counts what the underlying writer reports as accepted.
func (c *CountingWriter) Write(p []byte) (int, error) { return 0, nil }

// Count reports the total bytes accepted so far.
func (c *CountingWriter) Count() int64 { return 0 }

// PrefixWriter writes prefix at the start of every line (Part 4). It holds
// state across Write calls, so lines may be split between writes.
type PrefixWriter struct{}

// NewPrefixWriter wraps w. An empty prefix passes data through unchanged.
func NewPrefixWriter(w io.Writer, prefix string) *PrefixWriter { return &PrefixWriter{} }

// Write implements io.Writer. It returns the number of input bytes consumed.
func (p *PrefixWriter) Write(b []byte) (int, error) { return 0, nil }

// HashTee writes to a sink and digests every byte the sink accepted (Part 4).
type HashTee struct{}

// NewHashTee wraps w and digests with each named algorithm: "md5", "sha1" or
// "sha256". An unknown name is an error.
func NewHashTee(w io.Writer, algorithms ...string) (*HashTee, error) {
	return nil, errors.New("not implemented")
}

// Write implements io.Writer.
func (h *HashTee) Write(p []byte) (int, error) { return 0, nil }

// Sums returns the lowercase hex digest per algorithm name.
func (h *HashTee) Sums() map[string]string { return nil }

// LimitWriter forwards at most a fixed number of bytes and then fails (Part 4).
type LimitWriter struct{}

// NewLimitWriter wraps w so that at most limit bytes pass through in total.
func NewLimitWriter(w io.Writer, limit int64) *LimitWriter { return &LimitWriter{} }

// Write implements io.Writer.
func (l *LimitWriter) Write(p []byte) (int, error) { return 0, nil }
