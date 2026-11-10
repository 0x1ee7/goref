package perf_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	perf "goref/exercises/20_performance"
)

// countingWriter counts Write calls so tests can check the one-write-per-batch rule.
type countingWriter struct {
	calls int
	buf   bytes.Buffer
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.calls++
	return w.buf.Write(p)
}

// failWriter fails every Write with err.
type failWriter struct{ err error }

func (w failWriter) Write([]byte) (int, error) { return 0, w.err }

func sampleRecords() []perf.Record {
	return []perf.Record{{ID: 1, Name: "alice"}, {ID: -2, Name: "bob"}, {ID: 3, Name: ""}}
}

func encodeOracle(recs []perf.Record) string {
	var b strings.Builder
	for _, r := range recs {
		fmt.Fprintf(&b, "%d\t%s\n", r.ID, r.Name)
	}
	return b.String()
}

func TestPart4WriteRecordsFormat(t *testing.T) {
	const want = "1\talice\n-2\tbob\n3\t\n"
	var buf bytes.Buffer
	n, err := perf.WriteRecords(&buf, sampleRecords())
	if err != nil {
		t.Fatalf("WriteRecords error = %v; want nil", err)
	}
	if got := buf.String(); got != want {
		t.Errorf("output = %q; want %q", got, want)
	}
	if n != int64(len(want)) {
		t.Errorf("WriteRecords returned n = %d; want %d", n, len(want))
	}
}

func TestPart4WriteRecordsSingleWrite(t *testing.T) {
	recs := make([]perf.Record, 1000)
	for i := range recs {
		recs[i] = perf.Record{ID: int64(i) * 1_000_003, Name: strings.Repeat("n", i%7)}
	}
	var w countingWriter
	n, err := perf.WriteRecords(&w, recs)
	if err != nil {
		t.Fatalf("WriteRecords error = %v; want nil", err)
	}
	if w.calls != 1 {
		t.Errorf("WriteRecords made %d Write calls for one batch; want 1", w.calls)
	}
	want := encodeOracle(recs)
	if got := w.buf.String(); got != want {
		t.Errorf("output does not match oracle (lengths got=%d want=%d)", len(got), len(want))
	}
	if n != int64(len(want)) {
		t.Errorf("WriteRecords returned n = %d; want %d", n, len(want))
	}
}

func TestPart4WriteRecordsLargeBatch(t *testing.T) {
	// Large enough to push the scratch buffer past the pool's retention limit.
	recs := make([]perf.Record, 20_000)
	for i := range recs {
		recs[i] = perf.Record{ID: int64(i), Name: strings.Repeat("x", i%97)}
	}
	var buf bytes.Buffer
	if _, err := perf.WriteRecords(&buf, recs); err != nil {
		t.Fatalf("WriteRecords error = %v; want nil", err)
	}
	if want := encodeOracle(recs); buf.String() != want {
		t.Errorf("large batch output does not match oracle (lengths got=%d want=%d)", buf.Len(), len(want))
	}
}

func TestPart4WriteRecordsEmptyBatch(t *testing.T) {
	var pre countingWriter
	if n, err := perf.WriteRecords(&pre, sampleRecords()); err != nil || n == 0 {
		t.Fatalf("precheck: WriteRecords(sampleRecords) = (%d, %v); want non-empty output", n, err)
	}
	var w countingWriter
	n, err := perf.WriteRecords(&w, nil)
	if err != nil || n != 0 {
		t.Errorf("WriteRecords(nil) = (%d, %v); want (0, nil)", n, err)
	}
	if w.calls != 0 {
		t.Errorf("WriteRecords(nil) made %d Write calls; want 0", w.calls)
	}
}

func TestPart4WriteRecordsRejectsBadNames(t *testing.T) {
	tests := []struct {
		name string
		recs []perf.Record
	}{
		{"tab", []perf.Record{{ID: 1, Name: "a\tb"}}},
		{"LF", []perf.Record{{ID: 1, Name: "line\nbreak"}}},
		{"CR", []perf.Record{{ID: 1, Name: "cr\rhere"}}},
		{"bad name after good records", []perf.Record{{ID: 1, Name: "ok"}, {ID: 2, Name: "bad\tname"}, {ID: 3, Name: "ok"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w countingWriter
			n, err := perf.WriteRecords(&w, tt.recs)
			if !errors.Is(err, perf.ErrBadName) {
				t.Errorf("WriteRecords error = %v; want ErrBadName", err)
			}
			if n != 0 {
				t.Errorf("WriteRecords returned n = %d; want 0", n)
			}
			if w.calls != 0 || w.buf.Len() != 0 {
				t.Errorf("WriteRecords wrote %d bytes in %d calls on error; want nothing written", w.buf.Len(), w.calls)
			}
		})
	}
}

func TestPart4WriteRecordsPropagatesWriterError(t *testing.T) {
	errDisk := errors.New("disk full")
	n, err := perf.WriteRecords(failWriter{err: errDisk}, sampleRecords())
	if !errors.Is(err, errDisk) {
		t.Errorf("WriteRecords error = %v; want %v", err, errDisk)
	}
	if n != 0 {
		t.Errorf("WriteRecords returned n = %d; want 0 (failWriter wrote nothing)", n)
	}
}

func TestPart4WriteRecordsConcurrent(t *testing.T) {
	recs := sampleRecords()
	want := strings.Repeat(encodeOracle(recs), 50)

	const writers = 8
	outs := make([]bytes.Buffer, writers)
	errs := make([]error, writers)
	var wg sync.WaitGroup
	for i := range writers {
		wg.Go(func() {
			for range 50 {
				if _, err := perf.WriteRecords(&outs[i], recs); err != nil {
					errs[i] = err
					return
				}
			}
		})
	}
	wg.Wait()

	for i := range writers {
		if errs[i] != nil {
			t.Errorf("writer %d: WriteRecords error = %v; want nil", i, errs[i])
			continue
		}
		if got := outs[i].String(); got != want {
			t.Errorf("writer %d: output corrupted (len got=%d want=%d)", i, len(got), len(want))
		}
	}
}

func TestPart4WriteRecordsZeroAllocs(t *testing.T) {
	recs := sampleRecords()
	want := encodeOracle(recs)
	if n, err := perf.WriteRecords(io.Discard, recs); err != nil || n != int64(len(want)) {
		t.Fatalf("WriteRecords(io.Discard) = (%d, %v); want (%d, nil)", n, err, len(want))
	}
	// AllocsPerRun averages with integer division, so a rare pool refill after a GC
	// does not fail the budget; a per-call allocation does.
	allocs := testing.AllocsPerRun(200, func() {
		sinkN, sinkErr = perf.WriteRecords(io.Discard, recs)
	})
	if allocs >= 1 {
		t.Errorf("WriteRecords allocated %v times per steady-state call; want 0 (use a sync.Pool scratch buffer)", allocs)
	}
}
