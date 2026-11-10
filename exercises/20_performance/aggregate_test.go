package perf_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	perf "goref/exercises/20_performance"
)

// toColumnsOracle builds Columns independently of perf.ToColumns.
func toColumnsOracle(orders []perf.Order) perf.Columns {
	var c perf.Columns
	for _, o := range orders {
		c.Customer = append(c.Customer, o.Customer)
		c.Cents = append(c.Cents, o.Cents)
		c.Cancelled = append(c.Cancelled, o.Cancelled)
	}
	return c
}

// oracleRevenue is a map-based reference for per-customer revenue.
func oracleRevenue(orders []perf.Order, customers int) []int64 {
	m := make(map[uint32]int64)
	for _, o := range orders {
		if !o.Cancelled {
			m[o.Customer] += o.Cents
		}
	}
	out := make([]int64, customers)
	for id, v := range m {
		out[id] = v
	}
	return out
}

var revenueImpls = []struct {
	name string
	fn   func([]perf.Order, int) ([]int64, error)
}{
	{"AoS", perf.RevenueAoS},
	{"SoA", func(orders []perf.Order, customers int) ([]int64, error) {
		return perf.RevenueSoA(toColumnsOracle(orders), customers)
	}},
}

func TestPart6ToColumns(t *testing.T) {
	orders := []perf.Order{{Customer: 2, Cents: 500}, {Customer: 0, Cents: -20, Cancelled: true}, {Customer: 2, Cents: 1}}
	want := perf.Columns{
		Customer:  []uint32{2, 0, 2},
		Cents:     []int64{500, -20, 1},
		Cancelled: []bool{false, true, false},
	}
	got := perf.ToColumns(orders)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("ToColumns mismatch (-want +got):\n%s", diff)
	}

	got.Cents[0] = 999
	if orders[0].Cents != 500 {
		t.Errorf("mutating ToColumns result changed the input; the layouts must not alias")
	}

	empty := perf.ToColumns(nil)
	if len(empty.Customer) != 0 || len(empty.Cents) != 0 || len(empty.Cancelled) != 0 {
		t.Errorf("ToColumns(nil) has lengths %d/%d/%d; want 0/0/0",
			len(empty.Customer), len(empty.Cents), len(empty.Cancelled))
	}
}

func TestPart6RevenueTable(t *testing.T) {
	tests := []struct {
		name      string
		orders    []perf.Order
		customers int
		want      []int64
		wantErr   error
	}{
		{"no orders", nil, 3, []int64{0, 0, 0}, nil},
		{"sums per customer", []perf.Order{{Customer: 0, Cents: 100}, {Customer: 2, Cents: 50}, {Customer: 0, Cents: 25}}, 3, []int64{125, 0, 50}, nil},
		{"cancelled orders excluded", []perf.Order{{Customer: 1, Cents: 100}, {Customer: 1, Cents: 999, Cancelled: true}}, 2, []int64{0, 100}, nil},
		{"negative amounts reduce revenue", []perf.Order{{Customer: 0, Cents: -10}, {Customer: 0, Cents: 30}}, 1, []int64{20}, nil},
		{"zero customers rejects any order", []perf.Order{{Customer: 0, Cents: 1}}, 0, nil, perf.ErrCustomerRange},
		{"customer out of range", []perf.Order{{Customer: 3, Cents: 1}}, 3, nil, perf.ErrCustomerRange},
		{"out-of-range customer on a cancelled order still rejected", []perf.Order{{Customer: 5, Cents: 1, Cancelled: true}}, 2, nil, perf.ErrCustomerRange},
		{"negative customer count", nil, -1, nil, perf.ErrCustomerRange},
	}
	for _, impl := range revenueImpls {
		for _, tt := range tests {
			t.Run(impl.name+"/"+tt.name, func(t *testing.T) {
				got, err := impl.fn(tt.orders, tt.customers)
				if tt.wantErr != nil {
					if !errors.Is(err, tt.wantErr) {
						t.Errorf("error = %v; want %v", err, tt.wantErr)
					}
					return
				}
				if err != nil {
					t.Fatalf("error = %v; want nil", err)
				}
				if diff := cmp.Diff(tt.want, got, cmpopts.EquateEmpty()); diff != "" {
					t.Errorf("revenue mismatch (-want +got):\n%s", diff)
				}
			})
		}
	}
}

func TestPart6RevenueSoARagged(t *testing.T) {
	tests := []struct {
		name string
		c    perf.Columns
	}{
		{"short Cents", perf.Columns{Customer: []uint32{0, 1}, Cents: []int64{1}, Cancelled: []bool{false, false}}},
		{"short Cancelled", perf.Columns{Customer: []uint32{0, 1}, Cents: []int64{1, 2}, Cancelled: []bool{false}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := perf.RevenueSoA(tt.c, 2)
			if !errors.Is(err, perf.ErrRagged) {
				t.Errorf("RevenueSoA error = %v; want ErrRagged", err)
			}
		})
	}
}

func TestPart6AoSAndSoAAgreeWithOracle(t *testing.T) {
	const customers = 257
	orders := genOrders(10_000, customers)
	want := oracleRevenue(orders, customers)

	aos, err := perf.RevenueAoS(orders, customers)
	if err != nil {
		t.Fatalf("RevenueAoS error = %v; want nil", err)
	}
	if diff := cmp.Diff(want, aos); diff != "" {
		t.Errorf("RevenueAoS mismatch (-want +got):\n%s", diff)
	}

	soa, err := perf.RevenueSoA(toColumnsOracle(orders), customers)
	if err != nil {
		t.Fatalf("RevenueSoA error = %v; want nil", err)
	}
	if diff := cmp.Diff(want, soa); diff != "" {
		t.Errorf("RevenueSoA mismatch (-want +got):\n%s", diff)
	}
}

func TestPart6AggregateAllocs(t *testing.T) {
	const customers = 16
	orders := genOrders(500, customers)
	cols := toColumnsOracle(orders)
	want := oracleRevenue(orders, customers)

	// Correctness first: a stub returning nil passes allocation budgets of zero.
	if got, err := perf.RevenueAoS(orders, customers); err != nil || len(got) != customers {
		t.Fatalf("RevenueAoS precheck: len=%d err=%v", len(got), err)
	}
	if got, err := perf.RevenueSoA(cols, customers); err != nil || len(got) != customers {
		t.Fatalf("RevenueSoA precheck: len=%d err=%v", len(got), err)
	}
	if c := perf.ToColumns(orders); len(c.Cents) != len(orders) {
		t.Fatalf("ToColumns precheck: len=%d; want %d", len(c.Cents), len(orders))
	}
	if got, _ := perf.RevenueAoS(orders, customers); !cmp.Equal(want, got) {
		t.Fatal("RevenueAoS precheck: values differ from oracle")
	}

	if n := testing.AllocsPerRun(100, func() { sinkRev, sinkErr = perf.RevenueAoS(orders, customers) }); n != 1 {
		t.Errorf("RevenueAoS allocated %v times per call; want exactly 1 (the result)", n)
	}
	if n := testing.AllocsPerRun(100, func() { sinkRev, sinkErr = perf.RevenueSoA(cols, customers) }); n != 1 {
		t.Errorf("RevenueSoA allocated %v times per call; want exactly 1 (the result)", n)
	}
	if n := testing.AllocsPerRun(100, func() { sinkCols = perf.ToColumns(orders) }); n != 3 {
		t.Errorf("ToColumns allocated %v times per call; want exactly 3 (one per column)", n)
	}
}
