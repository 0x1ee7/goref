package perf_test

import (
	"bytes"
	"errors"
	"math"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"

	perf "goref/exercises/20_performance"
)

func TestPart1ParseLineFields(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want perf.Line
	}{
		{
			name: "canonical",
			in:   "level=info route=/api/orders status=200 dur_ms=12",
			want: perf.Line{Level: []byte("info"), Route: []byte("/api/orders"), Status: 200, DurMS: 12},
		},
		{
			name: "fields in any order",
			in:   "dur_ms=7 status=503 route=/health level=error",
			want: perf.Line{Level: []byte("error"), Route: []byte("/health"), Status: 503, DurMS: 7},
		},
		{
			name: "unknown keys ignored, including empty values",
			in:   "ts=2026-11-10T12:00:00Z request_id= level=warn route=/a status=404 dur_ms=0 ua=curl/8",
			want: perf.Line{Level: []byte("warn"), Route: []byte("/a"), Status: 404, DurMS: 0},
		},
		{
			name: "runs of spaces separate tokens",
			in:   "level=info   route=/x  status=200 dur_ms=1",
			want: perf.Line{Level: []byte("info"), Route: []byte("/x"), Status: 200, DurMS: 1},
		},
		{
			name: "trailing CRLF is not part of the value",
			in:   "level=info route=/ status=204 dur_ms=3\r\n",
			want: perf.Line{Level: []byte("info"), Route: []byte("/"), Status: 204, DurMS: 3},
		},
		{
			name: "duplicate key: last wins",
			in:   "level=info route=/a status=500 status=201 dur_ms=5",
			want: perf.Line{Level: []byte("info"), Route: []byte("/a"), Status: 201, DurMS: 5},
		},
		{
			name: "largest int",
			in:   "level=info route=/ status=" + strconv.Itoa(math.MaxInt) + " dur_ms=0",
			want: perf.Line{Level: []byte("info"), Route: []byte("/"), Status: math.MaxInt, DurMS: 0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := perf.ParseLine([]byte(tt.in))
			if err != nil {
				t.Fatalf("ParseLine(%q) error = %v; want nil", tt.in, err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ParseLine(%q) mismatch (-want +got):\n%s", tt.in, diff)
			}
		})
	}
}

func TestPart1ParseLineErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want error
	}{
		{"empty line", "", perf.ErrMissingField},
		{"missing level", "route=/a status=200 dur_ms=1", perf.ErrMissingField},
		{"missing route", "level=info status=200 dur_ms=1", perf.ErrMissingField},
		{"missing status", "level=info route=/a dur_ms=1", perf.ErrMissingField},
		{"missing dur_ms", "level=info route=/a status=200", perf.ErrMissingField},
		{"empty level value", "level= route=/a status=200 dur_ms=1", perf.ErrBadValue},
		{"empty route value", "level=info route= status=200 dur_ms=1", perf.ErrBadValue},
		{"non-numeric status", "level=info route=/a status=2x0 dur_ms=1", perf.ErrBadValue},
		{"negative status", "level=info route=/a status=-1 dur_ms=1", perf.ErrBadValue},
		{"explicit plus sign", "level=info route=/a status=+200 dur_ms=1", perf.ErrBadValue},
		{"underscore separator", "level=info route=/a status=200 dur_ms=1_000", perf.ErrBadValue},
		{"empty dur_ms", "level=info route=/a status=200 dur_ms=", perf.ErrBadValue},
		{"int overflow", "level=info route=/a status=999999999999999999999999999999 dur_ms=1", perf.ErrBadValue},
		{"token without equals", "level=info route=/a status=200 dur_ms=1 flush", perf.ErrSyntax},
		{"empty key", "level=info =x route=/a status=200 dur_ms=1", perf.ErrSyntax},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := perf.ParseLine([]byte(tt.in))
			if !errors.Is(err, tt.want) {
				t.Errorf("ParseLine(%q) error = %v; want %v", tt.in, err, tt.want)
			}
		})
	}
}

func TestPart1ParseLineViewsInput(t *testing.T) {
	buf := []byte("level=warn route=/aliased status=200 dur_ms=1")
	got, err := perf.ParseLine(buf)
	if err != nil {
		t.Fatalf("ParseLine error = %v; want nil", err)
	}
	i := bytes.Index(buf, []byte("warn"))
	if i < 0 {
		t.Fatal("test setup: \"warn\" not found in input")
	}
	if len(got.Level) == 0 || &got.Level[0] != &buf[i] {
		t.Fatal("Level does not alias the input buffer; ParseLine must return views, not copies")
	}
	buf[i] = 'W'
	if string(got.Level) != "Warn" {
		t.Errorf("after mutating the input, Level = %q; want %q", got.Level, "Warn")
	}
}

func TestPart1ParseLineZeroAllocs(t *testing.T) {
	lines := []string{
		"level=info route=/api/orders status=200 dur_ms=12",
		"ts=x level=error route=/api/orders/42 status=503 dur_ms=1500 ua=curl",
		"dur_ms=0 status=204 route=/health level=debug",
	}
	want := []perf.Line{
		{Level: []byte("info"), Route: []byte("/api/orders"), Status: 200, DurMS: 12},
		{Level: []byte("error"), Route: []byte("/api/orders/42"), Status: 503, DurMS: 1500},
		{Level: []byte("debug"), Route: []byte("/health"), Status: 204, DurMS: 0},
	}
	bufs := make([][]byte, len(lines))
	for i, s := range lines {
		bufs[i] = []byte(s)
		got, err := perf.ParseLine(bufs[i])
		if err != nil {
			t.Fatalf("ParseLine(%q) error = %v; want nil", s, err)
		}
		if diff := cmp.Diff(want[i], got); diff != "" {
			t.Fatalf("ParseLine(%q) mismatch (-want +got):\n%s", s, diff)
		}
	}
	allocs := testing.AllocsPerRun(1000, func() {
		for _, b := range bufs {
			sinkLine, sinkErr = perf.ParseLine(b)
		}
	})
	if allocs != 0 {
		t.Errorf("parsing %d lines allocated %v times per run; want 0", len(bufs), allocs)
	}
}
