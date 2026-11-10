package iterators_test

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	iterators "goref/exercises/05_iterators"
)

// splitSemicolon is a custom bufio.SplitFunc: tokens are separated by ';'.
func splitSemicolon(data []byte, atEOF bool) (int, []byte, error) {
	if i := bytes.IndexByte(data, ';'); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// trickle hands out one byte per Read and counts the bytes it has given away,
// so a test can observe exactly how far a consumer has read.
type trickle struct {
	r    io.Reader
	read int
}

func (t *trickle) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	n, err := t.r.Read(p)
	t.read += n
	return n, err
}

func TestPart4Tokens(t *testing.T) {
	tests := []struct {
		name  string
		input string
		split bufio.SplitFunc
		want  []string
	}{
		{"words", "  one two\tthree\n", bufio.ScanWords, []string{"one", "two", "three"}},
		{"custom separator", "x;y;z;", splitSemicolon, []string{"x", "y", "z"}},
		{"custom, no trailing separator", "x;y;z", splitSemicolon, []string{"x", "y", "z"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := drain(t, iterators.Tokens(strings.NewReader(tc.input), tc.split, 0))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Tokens mismatch (-want +got):\n%s", diff)
			}
		})
	}

	t.Run("overlong token is an error, then iteration ends", func(t *testing.T) {
		input := "ok;" + strings.Repeat("x", 100) + ";after"
		got, err := drain(t, iterators.Tokens(strings.NewReader(input), splitSemicolon, 16))
		if diff := cmp.Diff([]string{"ok"}, got); diff != "" {
			t.Errorf("values mismatch (-want +got):\n%s", diff)
		}
		if !errors.Is(err, bufio.ErrTooLong) {
			t.Errorf("err = %v, want bufio.ErrTooLong", err)
		}
	})
}

func TestPart4Lines(t *testing.T) {
	t.Run("terminators and blank lines", func(t *testing.T) {
		got, err := drain(t, iterators.Lines(strings.NewReader("alpha\r\nbeta\n\ngamma"), 64))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if diff := cmp.Diff([]string{"alpha", "beta", "", "gamma"}, got); diff != "" {
			t.Errorf("Lines mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("zero limit means bufio.MaxScanTokenSize", func(t *testing.T) {
		long := strings.Repeat("a", 70_000)
		got, err := drain(t, iterators.Lines(strings.NewReader(long+"\n"), 0))
		if !errors.Is(err, bufio.ErrTooLong) {
			t.Errorf("err = %v, want bufio.ErrTooLong for a 70000-byte line with default limit", err)
		}
		if len(got) != 0 {
			t.Errorf("got %d lines before the error, want 0", len(got))
		}
	})

	t.Run("explicit limit admits the long line", func(t *testing.T) {
		long := strings.Repeat("a", 70_000)
		got, err := drain(t, iterators.Lines(strings.NewReader(long+"\n"), 100_000))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0] != long {
			t.Errorf("got %d lines, want the one 70000-byte line", len(got))
		}
	})

	t.Run("overlong line yields error and ends", func(t *testing.T) {
		input := "ok\n" + strings.Repeat("x", 100) + "\nafter\n"
		got, err := drain(t, iterators.Lines(strings.NewReader(input), 16))
		if diff := cmp.Diff([]string{"ok"}, got); diff != "" {
			t.Errorf("values mismatch (-want +got):\n%s", diff)
		}
		if !errors.Is(err, bufio.ErrTooLong) {
			t.Errorf("err = %v, want bufio.ErrTooLong", err)
		}
	})

	t.Run("read error is surfaced", func(t *testing.T) {
		r := io.MultiReader(strings.NewReader("ok\n"), errReader{errBoom})
		got, err := drain(t, iterators.Lines(r, 64))
		if diff := cmp.Diff([]string{"ok"}, got); diff != "" {
			t.Errorf("values mismatch (-want +got):\n%s", diff)
		}
		if !errors.Is(err, errBoom) {
			t.Errorf("err = %v, want errBoom", err)
		}
	})
}

func TestPart4Entries(t *testing.T) {
	t.Run("skips blanks but counts physical lines", func(t *testing.T) {
		input := "INFO boot ok\n\n   \nWARN disk 91%\nERROR db down\n"
		got, err := drain(t, iterators.Entries(iterators.Lines(strings.NewReader(input), 64)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []iterators.Entry{
			{Lineno: 1, Level: "INFO", Message: "boot ok"},
			{Lineno: 4, Level: "WARN", Message: "disk 91%"},
			{Lineno: 5, Level: "ERROR", Message: "db down"},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Entries mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("unknown level yields wrapped error with line number and ends", func(t *testing.T) {
		input := "INFO a\nTRACE x\nERROR y\n"
		got, err := drain(t, iterators.Entries(iterators.Lines(strings.NewReader(input), 64)))
		if diff := cmp.Diff([]iterators.Entry{{Lineno: 1, Level: "INFO", Message: "a"}}, got); diff != "" {
			t.Errorf("values mismatch (-want +got):\n%s", diff)
		}
		if !errors.Is(err, iterators.ErrUnknownLevel) {
			t.Fatalf("err = %v, want ErrUnknownLevel", err)
		}
		if !strings.Contains(err.Error(), "line 2") {
			t.Errorf("err = %q, want it to name line 2", err)
		}
	})

	t.Run("upstream error passes through", func(t *testing.T) {
		r := io.MultiReader(strings.NewReader("INFO a\n"), errReader{errBoom})
		got, err := drain(t, iterators.Entries(iterators.Lines(r, 64)))
		if diff := cmp.Diff([]iterators.Entry{{Lineno: 1, Level: "INFO", Message: "a"}}, got); diff != "" {
			t.Errorf("values mismatch (-want +got):\n%s", diff)
		}
		if !errors.Is(err, errBoom) {
			t.Errorf("err = %v, want errBoom", err)
		}
	})
}

func TestPart4LazyReads(t *testing.T) {
	const total = 1000
	lines := make([]string, total)
	for i := range lines {
		lines[i] = fmt.Sprintf("INFO line %d\n", i+1)
	}
	input := strings.Join(lines, "")

	t.Run("Lines reads only through the last consumed newline", func(t *testing.T) {
		tr := &trickle{r: strings.NewReader(input)}
		var got []string
		for line, err := range iterators.Lines(tr, 64) {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got = append(got, line)
			if len(got) == 1 {
				break
			}
		}
		if diff := cmp.Diff([]string{"INFO line 1"}, got); diff != "" {
			t.Errorf("got mismatch (-want +got):\n%s", diff)
		}
		if want := len(lines[0]); tr.read != want {
			t.Errorf("reader consumed %d bytes, want %d", tr.read, want)
		}
	})

	t.Run("Entries pipeline reads only what the consumer needs", func(t *testing.T) {
		tr := &trickle{r: strings.NewReader(input)}
		var got []iterators.Entry
		for e, err := range iterators.Entries(iterators.Lines(tr, 64)) {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got = append(got, e)
			if len(got) == 3 {
				break
			}
		}
		if len(got) != 3 || got[2].Lineno != 3 {
			t.Fatalf("got %d entries, want 3 ending at line 3", len(got))
		}
		if want := len(lines[0]) + len(lines[1]) + len(lines[2]); tr.read != want {
			t.Errorf("reader consumed %d bytes, want %d", tr.read, want)
		}
	})
}
