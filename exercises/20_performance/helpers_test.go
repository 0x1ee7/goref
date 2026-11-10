package perf_test

import (
	rand "math/rand/v2"

	perf "goref/exercises/20_performance"
)

// Package-level sinks keep results live so the compiler cannot elide the work
// measured by testing.AllocsPerRun and the benchmarks.
var (
	sinkLine   perf.Line
	sinkErr    error
	sinkStr    string
	sinkBytes  []byte
	sinkInts   []int
	sinkChunks [][]int
	sinkN      int64
	sinkRev    []int64
	sinkCols   perf.Columns
)

// genOrders returns n deterministic orders spread over the given customer count.
func genOrders(n, customers int) []perf.Order {
	rng := rand.New(rand.NewPCG(42, 7))
	orders := make([]perf.Order, n)
	for i := range orders {
		orders[i] = perf.Order{
			Customer:  uint32(rng.IntN(customers)),
			Cents:     int64(rng.IntN(200_000)) - 1_000,
			Cancelled: rng.IntN(5) == 0,
		}
	}
	return orders
}
