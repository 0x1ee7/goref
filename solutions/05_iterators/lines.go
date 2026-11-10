package iterators

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"iter"
	"strings"
)

// ErrUnknownLevel reports a log line whose level is not DEBUG, INFO, WARN or ERROR.
var ErrUnknownLevel = errors.New("unknown log level")

// Entry is one parsed log line.
type Entry struct {
	// Lineno is the 1-based physical line number in the input.
	Lineno int
	Level  string
	// Message is the rest of the line after the level.
	Message string
}

// Tokens yields the tokens of r as produced by split. Each token is a string
// copy, so it stays valid after iteration moves on. A non-positive
// maxTokenSize means bufio.MaxScanTokenSize. A scanner error is yielded once
// as ("", err), and iteration ends.
func Tokens(r io.Reader, split bufio.SplitFunc, maxTokenSize int) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		sc := bufio.NewScanner(r)
		sc.Split(split)
		if maxTokenSize > 0 {
			sc.Buffer(nil, maxTokenSize)
		}
		for sc.Scan() {
			if !yield(sc.Text(), nil) {
				return
			}
		}
		if err := sc.Err(); err != nil {
			yield("", err)
		}
	}
}

// Lines yields the lines of r without their \n or \r\n terminator. A
// non-positive maxLineBytes means bufio.MaxScanTokenSize.
func Lines(r io.Reader, maxLineBytes int) iter.Seq2[string, error] {
	return Tokens(r, bufio.ScanLines, maxLineBytes)
}

// Entries parses "LEVEL message" lines from lines. Blank lines are skipped but
// still counted, so Lineno matches the physical input. An unknown level yields
// a wrapped ErrUnknownLevel and ends iteration. An upstream error is passed
// through and also ends iteration.
func Entries(lines iter.Seq2[string, error]) iter.Seq2[Entry, error] {
	return func(yield func(Entry, error) bool) {
		n := 0
		for line, err := range lines {
			n++
			if err != nil {
				yield(Entry{}, err)
				return
			}
			text := strings.TrimSpace(line)
			if text == "" {
				continue
			}
			level, msg, _ := strings.Cut(text, " ")
			switch level {
			case "DEBUG", "INFO", "WARN", "ERROR":
			default:
				yield(Entry{}, fmt.Errorf("line %d: %w", n, ErrUnknownLevel))
				return
			}
			e := Entry{Lineno: n, Level: level, Message: strings.TrimSpace(msg)}
			if !yield(e, nil) {
				return
			}
		}
	}
}
