package perf_test

import (
	"encoding/csv"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	perf "goref/exercises/20_performance"
)

func TestPart2CSVRecordExact(t *testing.T) {
	tests := []struct {
		name   string
		fields []string
		want   string
	}{
		{"plain", []string{"id", "name"}, "id,name"},
		{"comma forces quotes", []string{"1", "hello, world"}, `1,"hello, world"`},
		{"quotes are doubled", []string{`say "hi"`}, `"say ""hi"""`},
		{"LF forces quotes", []string{"a\nb"}, "\"a\nb\""},
		{"CR forces quotes", []string{"a\rb"}, "\"a\rb\""},
		{"spaces alone do not force quotes", []string{" x "}, " x "},
		{"empty fields", []string{"", ""}, ","},
		{"leading empty field", []string{"", "x"}, ",x"},
		{"empty field last", []string{"x", ""}, "x,"},
		{"non-ASCII is not special", []string{"héllo"}, "héllo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := perf.CSVRecord(tt.fields); got != tt.want {
				t.Errorf("CSVRecord(%q) = %q; want %q", tt.fields, got, tt.want)
			}
		})
	}
}

func TestPart2CSVRecordRoundTrip(t *testing.T) {
	// encoding/csv is the oracle for the reading side of RFC 4180.
	rows := [][]string{
		{"a", "b,c", `"d"`, "e\nf", "g\rh", "", " i "},
		{"x"},
		{"", "", ""},
	}
	for _, row := range rows {
		enc := perf.CSVRecord(row)
		got, err := csv.NewReader(strings.NewReader(enc + "\n")).Read()
		if err != nil {
			t.Fatalf("csv.Read(%q) error = %v; want nil", enc, err)
		}
		if diff := cmp.Diff(row, got); diff != "" {
			t.Errorf("round trip of %q mismatch (-want +got):\n%s", enc, diff)
		}
	}
}

func TestPart2AppendCSVRecord(t *testing.T) {
	fields := []string{"a", `b"c`, "d,e"}
	want := `a,"b""c","d,e"`
	if got := string(perf.AppendCSVRecord(nil, fields)); got != want {
		t.Errorf("AppendCSVRecord(nil, %q) = %q; want %q", fields, got, want)
	}
	if got := string(perf.AppendCSVRecord([]byte("row: "), fields)); got != "row: "+want {
		t.Errorf("AppendCSVRecord with prefix = %q; want %q", got, "row: "+want)
	}
}

func TestPart2QueryStringExact(t *testing.T) {
	tests := []struct {
		name   string
		params []perf.Param
		want   string
	}{
		{"single empty key and value", []perf.Param{{Key: "", Value: ""}}, "="},
		{"space becomes plus", []perf.Param{{Key: "q", Value: "go lang"}}, "q=go+lang"},
		{"order is preserved", []perf.Param{{Key: "b", Value: "2"}, {Key: "a", Value: "1"}}, "b=2&a=1"},
		{"reserved characters are escaped", []perf.Param{{Key: "a&b", Value: "x=y+z"}}, "a%26b=x%3Dy%2Bz"},
		{"unreserved characters are kept", []perf.Param{{Key: "k", Value: "~-_."}}, "k=~-_."},
		{"empty value", []perf.Param{{Key: "empty", Value: ""}}, "empty="},
		{
			name:   "path and unicode",
			params: []perf.Param{{Key: "next", Value: "/orders?page=2#top"}, {Key: "name", Value: "é"}},
			want:   "next=%2Forders%3Fpage%3D2%23top&name=%C3%A9",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := perf.QueryString(tt.params); got != tt.want {
				t.Errorf("QueryString(%v) = %q; want %q", tt.params, got, tt.want)
			}
		})
	}
}

func TestPart2QueryStringMatchesQueryEscape(t *testing.T) {
	for c := range 256 {
		s := string([]byte{byte(c)})
		got := perf.QueryString([]perf.Param{{Key: s, Value: s}})
		want := url.QueryEscape(s) + "=" + url.QueryEscape(s)
		if got != want {
			t.Errorf("byte %#02x: QueryString = %q; want %q", c, got, want)
		}
	}
}

func TestPart2BuildAllocs(t *testing.T) {
	fields := []string{"order-1001", "Ada Lovelace", "ada@example.com", "1234.50", `note "rush", fragile`}
	wantCSV := `order-1001,Ada Lovelace,ada@example.com,1234.50,"note ""rush"", fragile"`
	params := []perf.Param{{Key: "q", Value: "go lang & more"}, {Key: "page", Value: "2"}, {Key: "sort", Value: "-created_at"}, {Key: "tag", Value: "a/b"}}
	wantQuery := "q=go+lang+%26+more&page=2&sort=-created_at&tag=a%2Fb"

	// Correctness first: an allocation budget alone passes for a stub that returns "".
	if got := perf.CSVRecord(fields); got != wantCSV {
		t.Fatalf("CSVRecord = %q; want %q", got, wantCSV)
	}
	if got := perf.QueryString(params); got != wantQuery {
		t.Fatalf("QueryString = %q; want %q", got, wantQuery)
	}
	buf := make([]byte, 0, len(wantCSV))
	if got := string(perf.AppendCSVRecord(buf[:0], fields)); got != wantCSV {
		t.Fatalf("AppendCSVRecord into sized buffer = %q; want %q", got, wantCSV)
	}

	if n := testing.AllocsPerRun(100, func() { sinkStr = perf.CSVRecord(fields) }); n != 1 {
		t.Errorf("CSVRecord allocated %v times per call; want exactly 1", n)
	}
	if n := testing.AllocsPerRun(100, func() { sinkBytes = perf.AppendCSVRecord(nil, fields) }); n != 1 {
		t.Errorf("AppendCSVRecord(nil, ...) allocated %v times per call; want exactly 1", n)
	}
	if n := testing.AllocsPerRun(100, func() { sinkBytes = perf.AppendCSVRecord(buf[:0], fields) }); n != 0 {
		t.Errorf("AppendCSVRecord into sufficient capacity allocated %v times per call; want 0", n)
	}
	if n := testing.AllocsPerRun(100, func() { sinkStr = perf.QueryString(params) }); n != 1 {
		t.Errorf("QueryString allocated %v times per call; want exactly 1", n)
	}
}
