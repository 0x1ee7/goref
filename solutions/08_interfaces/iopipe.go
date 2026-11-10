package interfaces

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
)

// ErrLimitExceeded is returned by LimitWriter once its budget is spent.
var ErrLimitExceeded = errors.New("write limit exceeded")

// CountingWriter counts the bytes its underlying writer accepted (Part 4).
type CountingWriter struct {
	w io.Writer
	n int64
}

// NewCountingWriter wraps w.
func NewCountingWriter(w io.Writer) *CountingWriter {
	return &CountingWriter{w: w}
}

// Write forwards p and counts what the underlying writer reports as accepted.
func (c *CountingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// Count reports the total bytes accepted so far.
func (c *CountingWriter) Count() int64 { return c.n }

// PrefixWriter writes prefix at the start of every line (Part 4). It holds
// state across Write calls, so lines may be split between writes.
type PrefixWriter struct {
	w      io.Writer
	prefix string
	bol    bool // true when the next byte starts a line
}

// NewPrefixWriter wraps w. An empty prefix passes data through unchanged.
func NewPrefixWriter(w io.Writer, prefix string) *PrefixWriter {
	return &PrefixWriter{w: w, prefix: prefix, bol: true}
}

// Write implements io.Writer. It returns the number of input bytes consumed,
// not the number of prefixed bytes emitted.
func (p *PrefixWriter) Write(b []byte) (int, error) {
	consumed := 0
	for len(b) > 0 {
		if p.bol && p.prefix != "" {
			if _, err := io.WriteString(p.w, p.prefix); err != nil {
				return consumed, err
			}
		}
		seg := b
		nl := bytes.IndexByte(b, '\n')
		if nl >= 0 {
			seg = b[:nl+1]
		}
		p.bol = false
		n, err := p.w.Write(seg)
		consumed += n
		if err != nil {
			p.bol = n > 0 && seg[n-1] == '\n'
			return consumed, err
		}
		p.bol = nl >= 0
		b = b[len(seg):]
	}
	return consumed, nil
}

// hashFuncs maps algorithm names to constructors.
var hashFuncs = map[string]func() hash.Hash{
	"md5":    md5.New,
	"sha1":   sha1.New,
	"sha256": sha256.New,
}

// HashTee writes to a sink and digests every byte the sink accepted (Part 4).
type HashTee struct {
	w      io.Writer
	hashes map[string]hash.Hash
}

// NewHashTee wraps w and digests with each named algorithm: "md5", "sha1" or
// "sha256". An unknown name is an error.
func NewHashTee(w io.Writer, algorithms ...string) (*HashTee, error) {
	h := &HashTee{w: w, hashes: make(map[string]hash.Hash, len(algorithms))}
	for _, name := range algorithms {
		newHash, ok := hashFuncs[name]
		if !ok {
			return nil, fmt.Errorf("hash tee: unknown algorithm %q", name)
		}
		h.hashes[name] = newHash()
	}
	return h, nil
}

// Write implements io.Writer. Only the bytes the sink accepted are digested.
func (h *HashTee) Write(p []byte) (int, error) {
	n, err := h.w.Write(p)
	for _, hs := range h.hashes {
		hs.Write(p[:n]) // hash.Hash.Write never fails
	}
	return n, err
}

// Sums returns the lowercase hex digest per algorithm name.
func (h *HashTee) Sums() map[string]string {
	sums := make(map[string]string, len(h.hashes))
	for name, hs := range h.hashes {
		sums[name] = hex.EncodeToString(hs.Sum(nil))
	}
	return sums
}

// LimitWriter forwards at most a fixed number of bytes and then fails (Part 4).
type LimitWriter struct {
	w         io.Writer
	remaining int64
}

// NewLimitWriter wraps w so that at most limit bytes pass through in total.
// A negative limit allows nothing.
func NewLimitWriter(w io.Writer, limit int64) *LimitWriter {
	return &LimitWriter{w: w, remaining: max(limit, 0)}
}

// Write implements io.Writer. A write that crosses the limit forwards the
// allowed prefix and returns ErrLimitExceeded with the count forwarded.
func (l *LimitWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if l.remaining <= 0 {
		return 0, ErrLimitExceeded
	}
	if int64(len(p)) <= l.remaining {
		n, err := l.w.Write(p)
		l.remaining -= int64(n)
		return n, err
	}
	n, err := l.w.Write(p[:l.remaining])
	l.remaining -= int64(n)
	if err != nil {
		return n, err
	}
	return n, ErrLimitExceeded
}
