package perf_test

import (
	"io"
	"testing"

	perf "goref/exercises/20_performance"
)

// Benchmarks are for manual runs:
//
//	go test ./exercises/20_performance/ -run '^$' -bench . -benchmem

func BenchmarkPart1ParseLine(b *testing.B) {
	line := []byte("level=info route=/api/orders/42 status=200 dur_ms=12")
	b.ReportAllocs()
	for b.Loop() {
		sinkLine, sinkErr = perf.ParseLine(line)
	}
}

func BenchmarkPart2CSVRecord(b *testing.B) {
	fields := []string{"order-1001", "Ada Lovelace", "ada@example.com", "1234.50", `note "rush", fragile`}
	b.ReportAllocs()
	for b.Loop() {
		sinkStr = perf.CSVRecord(fields)
	}
}

func BenchmarkPart2QueryString(b *testing.B) {
	params := []perf.Param{{Key: "q", Value: "go lang & more"}, {Key: "page", Value: "2"}, {Key: "sort", Value: "-created_at"}}
	b.ReportAllocs()
	for b.Loop() {
		sinkStr = perf.QueryString(params)
	}
}

func BenchmarkPart3Map(b *testing.B) {
	xs := make([]int, 1000)
	b.ReportAllocs()
	for b.Loop() {
		sinkInts = perf.Map(xs, double)
	}
}

func BenchmarkPart3Batch(b *testing.B) {
	xs := make([]int, 1000)
	b.ReportAllocs()
	for b.Loop() {
		sinkChunks = perf.Batch(xs, 10)
	}
}

func BenchmarkPart4WriteRecords(b *testing.B) {
	recs := make([]perf.Record, 100)
	for i := range recs {
		recs[i] = perf.Record{ID: int64(i), Name: "record-name"}
	}
	b.ReportAllocs()
	for b.Loop() {
		sinkN, sinkErr = perf.WriteRecords(io.Discard, recs)
	}
}

func BenchmarkPart5AppendSample(b *testing.B) {
	s := perf.Sample{ID: 9001, Latency: 250_000, Status: 200, Retries: 1, OK: true}
	buf := make([]byte, 0, 128)
	b.ReportAllocs()
	for b.Loop() {
		buf = perf.AppendSample(buf[:0], s)
	}
}

func BenchmarkPart6RevenueAoS(b *testing.B) {
	orders := genOrders(100_000, 1000)
	b.ReportAllocs()
	for b.Loop() {
		sinkRev, sinkErr = perf.RevenueAoS(orders, 1000)
	}
}

func BenchmarkPart6RevenueSoA(b *testing.B) {
	cols := toColumnsOracle(genOrders(100_000, 1000))
	b.ReportAllocs()
	for b.Loop() {
		sinkRev, sinkErr = perf.RevenueSoA(cols, 1000)
	}
}
