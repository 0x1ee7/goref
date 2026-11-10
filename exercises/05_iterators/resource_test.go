package iterators_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	iterators "goref/exercises/05_iterators"
)

var (
	errOpen  = errors.New("open failed")
	errClose = errors.New("close failed")
)

// fakeSource is an io.ReadCloser that counts how often it is closed.
type fakeSource struct {
	io.Reader
	closes   *int
	closeErr error
}

func (f *fakeSource) Close() error {
	*f.closes++
	return f.closeErr
}

// rbFixture drives ReadBatches over an in-memory source and counts opens/closes.
type rbFixture struct {
	reader   io.Reader
	openErr  error
	closeErr error
	opens    int
	closes   int
}

func (f *rbFixture) open() (io.ReadCloser, error) {
	f.opens++
	if f.openErr != nil {
		return nil, f.openErr
	}
	return &fakeSource{Reader: f.reader, closes: &f.closes, closeErr: f.closeErr}, nil
}

func TestPart6ReadBatches(t *testing.T) {
	tests := []struct {
		name      string
		reader    io.Reader
		size      int
		openErr   error
		closeErr  error
		want      [][]string
		wantErr   error
		wantOpens int
		// closes is always expected to equal opens when the source was opened.
	}{
		{
			name:      "short tail batch",
			reader:    strings.NewReader("a\nb\nc\nd\ne\n"),
			size:      2,
			want:      [][]string{{"a", "b"}, {"c", "d"}, {"e"}},
			wantOpens: 1,
		},
		{
			name:      "last line without newline",
			reader:    strings.NewReader("a\nb\nc"),
			size:      2,
			want:      [][]string{{"a", "b"}, {"c"}},
			wantOpens: 1,
		},
		{
			name:      "size one",
			reader:    strings.NewReader("x\ny\n"),
			size:      1,
			want:      [][]string{{"x"}, {"y"}},
			wantOpens: 1,
		},
		{
			name:      "size zero is rejected without opening",
			reader:    strings.NewReader("a\n"),
			size:      0,
			wantErr:   iterators.ErrBatchSize,
			wantOpens: 0,
		},
		{
			name:      "open failure",
			reader:    strings.NewReader("a\n"),
			size:      2,
			openErr:   errOpen,
			wantErr:   errOpen,
			wantOpens: 1,
		},
		{
			name:      "close failure after a complete read",
			reader:    strings.NewReader("a\n"),
			size:      5,
			closeErr:  errClose,
			want:      [][]string{{"a"}},
			wantErr:   errClose,
			wantOpens: 1,
		},
		{
			name:      "read failure discards the partial batch",
			reader:    io.MultiReader(strings.NewReader("a\nb\nc"), errReader{errBoom}),
			size:      2,
			want:      [][]string{{"a", "b"}},
			wantErr:   errBoom,
			wantOpens: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &rbFixture{reader: tc.reader, openErr: tc.openErr, closeErr: tc.closeErr}
			got, err := drain(t, iterators.ReadBatches(f.open, tc.size))
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("batches mismatch (-want +got):\n%s", diff)
			}
			if !errors.Is(err, tc.wantErr) || (err == nil) != (tc.wantErr == nil) {
				t.Errorf("err = %v, want %v", err, tc.wantErr)
			}
			if f.opens != tc.wantOpens {
				t.Errorf("opened %d times, want %d", f.opens, tc.wantOpens)
			}
			if f.closes != f.opens-boolToInt(tc.openErr != nil) {
				t.Errorf("closed %d times for %d opens; every opened source must be closed exactly once", f.closes, f.opens)
			}
		})
	}

	t.Run("open is deferred until the first advance", func(t *testing.T) {
		f := &rbFixture{reader: strings.NewReader("a\nb\nc\nd\n")}
		seq := iterators.ReadBatches(f.open, 2)
		if f.opens != 0 {
			t.Fatalf("opened %d times before iteration, want 0", f.opens)
		}
		var got [][]string
		for batch, err := range seq {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got = append(got, batch)
			if f.opens != 1 {
				t.Fatalf("opened %d times during first batch, want 1", f.opens)
			}
			break
		}
		if diff := cmp.Diff([][]string{{"a", "b"}}, got); diff != "" {
			t.Errorf("batch mismatch (-want +got):\n%s", diff)
		}
		if f.closes != 1 {
			t.Errorf("closed %d times after consumer break, want 1", f.closes)
		}
	})
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
