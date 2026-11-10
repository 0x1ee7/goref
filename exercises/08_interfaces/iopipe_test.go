package interfaces_test

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"goref/exercises/08_interfaces"
)

var errSinkFull = errors.New("sink full")

// failingSink accepts at most limit bytes in total, then fails.
type failingSink struct {
	buf   bytes.Buffer
	limit int
}

func (f *failingSink) Write(p []byte) (int, error) {
	room := f.limit - f.buf.Len()
	if room <= 0 {
		return 0, errSinkFull
	}
	if len(p) <= room {
		return f.buf.Write(p)
	}
	n, _ := f.buf.Write(p[:room])
	return n, errSinkFull
}

func hexSum(sum []byte) string { return hex.EncodeToString(sum) }

func TestPart4CountingWriter(t *testing.T) {
	t.Run("counts every byte for a healthy sink", func(t *testing.T) {
		var buf bytes.Buffer
		cw := interfaces.NewCountingWriter(&buf)
		if n, err := cw.Write([]byte("abc")); n != 3 || err != nil {
			t.Fatalf("Write = %d, %v; want 3, nil", n, err)
		}
		if n, err := cw.Write([]byte("de")); n != 2 || err != nil {
			t.Fatalf("Write = %d, %v; want 2, nil", n, err)
		}
		if got := cw.Count(); got != 5 {
			t.Errorf("Count = %d, want 5", got)
		}
	})

	t.Run("counts only accepted bytes on a short write", func(t *testing.T) {
		cw := interfaces.NewCountingWriter(&failingSink{limit: 3})
		n, err := cw.Write([]byte("abcdef"))
		if n != 3 || !errors.Is(err, errSinkFull) {
			t.Fatalf("Write = %d, %v; want 3, errSinkFull", n, err)
		}
		if got := cw.Count(); got != 3 {
			t.Errorf("Count = %d, want 3", got)
		}
	})

	t.Run("io.Copy streams and counts", func(t *testing.T) {
		var buf bytes.Buffer
		cw := interfaces.NewCountingWriter(&buf)
		big := strings.Repeat("x", 10000)
		if _, err := io.Copy(cw, strings.NewReader(big)); err != nil {
			t.Fatalf("io.Copy: %v", err)
		}
		if got := cw.Count(); got != 10000 {
			t.Errorf("Count = %d, want 10000", got)
		}
		if buf.Len() != 10000 {
			t.Errorf("sink received %d bytes, want 10000", buf.Len())
		}
	})
}

func TestPart4PrefixWriter(t *testing.T) {
	run := func(t *testing.T, prefix string, chunks ...string) string {
		t.Helper()
		var buf bytes.Buffer
		pw := interfaces.NewPrefixWriter(&buf, prefix)
		for _, c := range chunks {
			n, err := pw.Write([]byte(c))
			if err != nil || n != len(c) {
				t.Fatalf("Write(%q) = %d, %v; want %d, nil", c, n, err, len(c))
			}
		}
		return buf.String()
	}

	cases := []struct {
		name   string
		prefix string
		chunks []string
		want   string
	}{
		{"each line gets the prefix", "> ", []string{"one\ntwo\n"}, "> one\n> two\n"},
		{"lines split across writes", "> ", []string{"he", "llo\nwo", "rld\n"}, "> hello\n> world\n"},
		{"partial last line is prefixed once", "> ", []string{"abc", "def\n"}, "> abcdef\n"},
		{"empty lines get the prefix", "> ", []string{"a\n\nb\n"}, "> a\n> \n> b\n"},
		{"no prefix after the final newline", "> ", []string{"x\n", ""}, "> x\n"},
		{"empty prefix passes data through", "", []string{"a\nb"}, "a\nb"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := run(t, tc.prefix, tc.chunks...); got != tc.want {
				t.Errorf("output = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("reports consumed input when the sink fails", func(t *testing.T) {
		pw := interfaces.NewPrefixWriter(&failingSink{limit: 0}, "> ")
		n, err := pw.Write([]byte("hi\n"))
		if !errors.Is(err, errSinkFull) || n != 0 {
			t.Errorf("Write on full sink = %d, %v; want 0, errSinkFull", n, err)
		}
		healthy := interfaces.NewPrefixWriter(&bytes.Buffer{}, "> ")
		if n, err := healthy.Write([]byte("hi\n")); n != 3 || err != nil {
			t.Errorf("Write on healthy sink = %d, %v; want 3, nil", n, err)
		}
	})

	t.Run("pipeline: counting over prefixing counts input bytes", func(t *testing.T) {
		var buf bytes.Buffer
		cw := interfaces.NewCountingWriter(interfaces.NewPrefixWriter(&buf, "| "))
		if _, err := io.Copy(cw, strings.NewReader("one\ntwo\n")); err != nil {
			t.Fatalf("io.Copy: %v", err)
		}
		if got := buf.String(); got != "| one\n| two\n" {
			t.Errorf("output = %q, want %q", got, "| one\n| two\n")
		}
		// PrefixWriter reports input bytes consumed (8), not the 12 bytes it emitted.
		if got := cw.Count(); got != 8 {
			t.Errorf("Count = %d, want 8", got)
		}
	})
}

func TestPart4HashTee(t *testing.T) {
	t.Run("passes bytes through and digests them", func(t *testing.T) {
		var buf bytes.Buffer
		tee, err := interfaces.NewHashTee(&buf, "sha256", "md5", "sha1")
		if err != nil {
			t.Fatalf("NewHashTee: %v", err)
		}
		if n, err := tee.Write([]byte("hello world")); n != 11 || err != nil {
			t.Fatalf("Write = %d, %v; want 11, nil", n, err)
		}
		if buf.String() != "hello world" {
			t.Errorf("sink = %q, want %q", buf.String(), "hello world")
		}
		s256, s1, m5 := sha256.Sum256([]byte("hello world")), sha1.Sum([]byte("hello world")), md5.Sum([]byte("hello world"))
		want := map[string]string{"sha256": hexSum(s256[:]), "sha1": hexSum(s1[:]), "md5": hexSum(m5[:])}
		if diff := cmp.Diff(want, tee.Sums()); diff != "" {
			t.Errorf("Sums mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("unknown algorithm is rejected", func(t *testing.T) {
		if _, err := interfaces.NewHashTee(&bytes.Buffer{}, "sha256"); err != nil {
			t.Fatalf("sanity: sha256 alone rejected: %v", err)
		}
		if _, err := interfaces.NewHashTee(&bytes.Buffer{}, "sha256", "whirlpool"); err == nil {
			t.Error("unknown algorithm: err = nil, want error")
		}
	})

	t.Run("hashes only the bytes the sink accepted", func(t *testing.T) {
		tee, err := interfaces.NewHashTee(&failingSink{limit: 3}, "sha256")
		if err != nil {
			t.Fatal(err)
		}
		n, err := tee.Write([]byte("abcdef"))
		if n != 3 || !errors.Is(err, errSinkFull) {
			t.Fatalf("Write = %d, %v; want 3, errSinkFull", n, err)
		}
		want := sha256.Sum256([]byte("abc"))
		if got := tee.Sums()["sha256"]; got != hexSum(want[:]) {
			t.Errorf("sha256 = %s, want digest of %q", got, "abc")
		}
	})

	t.Run("no algorithms means no sums but writes still pass", func(t *testing.T) {
		var buf bytes.Buffer
		tee, err := interfaces.NewHashTee(&buf)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tee.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
		if buf.String() != "x" {
			t.Errorf("sink = %q, want %q", buf.String(), "x")
		}
		if len(tee.Sums()) != 0 {
			t.Errorf("Sums = %v, want empty", tee.Sums())
		}
	})

	t.Run("placed after a prefixer it digests the prefixed output", func(t *testing.T) {
		var buf bytes.Buffer
		tee, err := interfaces.NewHashTee(&buf, "sha256")
		if err != nil {
			t.Fatal(err)
		}
		pw := interfaces.NewPrefixWriter(tee, "> ")
		if _, err := pw.Write([]byte("a\nb\n")); err != nil {
			t.Fatal(err)
		}
		want := sha256.Sum256(buf.Bytes())
		if got := tee.Sums()["sha256"]; got != hexSum(want[:]) {
			t.Errorf("sha256 = %s, want digest of the emitted %q", got, buf.String())
		}
	})
}

func TestPart4LimitWriter(t *testing.T) {
	t.Run("accepts exactly up to the limit", func(t *testing.T) {
		var buf bytes.Buffer
		lw := interfaces.NewLimitWriter(&buf, 5)
		if n, err := lw.Write([]byte("hello")); n != 5 || err != nil {
			t.Fatalf("Write = %d, %v; want 5, nil", n, err)
		}
		if buf.String() != "hello" {
			t.Errorf("sink = %q, want %q", buf.String(), "hello")
		}
	})

	t.Run("truncates at the limit and reports ErrLimitExceeded", func(t *testing.T) {
		var buf bytes.Buffer
		lw := interfaces.NewLimitWriter(&buf, 5)
		if n, err := lw.Write([]byte("hel")); n != 3 || err != nil {
			t.Fatalf("first Write = %d, %v; want 3, nil", n, err)
		}
		if n, err := lw.Write([]byte("lo world")); n != 2 || !errors.Is(err, interfaces.ErrLimitExceeded) {
			t.Fatalf("second Write = %d, %v; want 2, ErrLimitExceeded", n, err)
		}
		if buf.String() != "hello" {
			t.Errorf("sink = %q, want %q", buf.String(), "hello")
		}
		if n, err := lw.Write([]byte("x")); n != 0 || !errors.Is(err, interfaces.ErrLimitExceeded) {
			t.Errorf("Write after limit = %d, %v; want 0, ErrLimitExceeded", n, err)
		}
		if n, err := lw.Write(nil); n != 0 || err != nil {
			t.Errorf("empty Write after limit = %d, %v; want 0, nil", n, err)
		}
	})
}
