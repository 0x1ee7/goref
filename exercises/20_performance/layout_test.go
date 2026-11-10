package perf_test

import (
	"fmt"
	"math"
	"testing"
	"unsafe"

	perf "goref/exercises/20_performance"
)

func sampleOracle(s perf.Sample) string {
	return fmt.Sprintf("id=%d latency=%d status=%d retries=%d ok=%t\n", s.ID, s.Latency, s.Status, s.Retries, s.OK)
}

func TestPart5SampleSize(t *testing.T) {
	if got := unsafe.Sizeof(perf.Sample{}); got != 24 {
		t.Errorf("unsafe.Sizeof(perf.Sample{}) = %d; want 24 (reorder the fields to remove padding)", got)
	}
}

func TestPart5AppendSampleFormat(t *testing.T) {
	tests := []struct {
		name string
		s    perf.Sample
	}{
		{"typical", perf.Sample{ID: 1, Latency: 12, Status: 200, Retries: 0, OK: true}},
		{"extremes", perf.Sample{ID: math.MaxUint64, Latency: math.MinInt64, Status: math.MaxUint16, Retries: math.MaxUint8, OK: false}},
		{"zero value", perf.Sample{}},
		{"negative latency", perf.Sample{ID: 42, Latency: -5, Status: 503, Retries: 3, OK: false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(perf.AppendSample(nil, tt.s))
			if want := sampleOracle(tt.s); got != want {
				t.Errorf("AppendSample = %q; want %q", got, want)
			}
		})
	}
}

func TestPart5AppendSamplePreservesPrefix(t *testing.T) {
	s := perf.Sample{ID: 7, Latency: 1, Status: 404, Retries: 2, OK: true}
	got := string(perf.AppendSample([]byte("log: "), s))
	if want := "log: " + sampleOracle(s); got != want {
		t.Errorf("AppendSample with prefix = %q; want %q", got, want)
	}
}

func TestPart5AppendSampleZeroAllocs(t *testing.T) {
	s := perf.Sample{ID: 9001, Latency: 250_000, Status: 200, Retries: 1, OK: true}
	buf := make([]byte, 0, 128)
	buf = perf.AppendSample(buf[:0], s)
	if got, want := string(buf), sampleOracle(s); got != want {
		t.Fatalf("AppendSample precheck = %q; want %q", got, want)
	}
	allocs := testing.AllocsPerRun(1000, func() {
		buf = perf.AppendSample(buf[:0], s)
	})
	if allocs != 0 {
		t.Errorf("AppendSample allocated %v times per call into a sized buffer; want 0 (avoid fmt: interface args box)", allocs)
	}
}
