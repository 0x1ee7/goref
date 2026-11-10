package iterators

import (
	"bufio"
	"errors"
	"io"
	"iter"
)

// ErrUnknownLevel is returned (wrapped, with the line number) by Entries for a
// line whose leading level token is not one of DEBUG, INFO, WARN, ERROR.
var ErrUnknownLevel = errors.New("unknown log level")

// Entry is one parsed log line.
type Entry struct {
	Lineno  int    // 1-based physical line number, counting blank lines
	Level   string // DEBUG, INFO, WARN or ERROR
	Message string // everything after the first space
}

// Tokens lazily scans r with split and yields each token as a string copy.
// The copy matters: bufio.Scanner reuses its buffer, so the bytes of a token
// are only valid until the next scan.
//
// maxTokenSize bounds a single token; if it is <= 0, bufio.MaxScanTokenSize is
// used. An overlong token (bufio.ErrTooLong) or a read error is yielded once as
// ("", err) and iteration ends. Clean end-of-input ends iteration without an
// error. Tokens are read on demand: a consumer that stops early causes no
// further reads from r.
func Tokens(r io.Reader, split bufio.SplitFunc, maxTokenSize int) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {}
}

// Lines is Tokens with bufio.ScanLines. Line terminators (\n or \r\n) are
// removed.
func Lines(r io.Reader, maxLineBytes int) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {}
}

// Entries parses "LEVEL message..." lines produced by lines. Blank or
// whitespace-only lines are skipped but still counted in Lineno. An unknown
// level yields (Entry{}, fmt.Errorf("line %d: %w", n, ErrUnknownLevel)) and
// ends iteration. An error from lines is passed through unchanged and ends
// iteration. Entries is lazy and forwards a consumer break upstream.
func Entries(lines iter.Seq2[string, error]) iter.Seq2[Entry, error] {
	return func(yield func(Entry, error) bool) {}
}
