package scope_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"goref/exercises/03_packages_and_scope"
)

// drainInts calls f n times and returns the results.
func drainInts(f func() int, n int) []int {
	out := make([]int, 0, n)
	for range n {
		out = append(out, f())
	}
	return out
}

func TestPart2Counter(t *testing.T) {
	t.Run("ascending from start", func(t *testing.T) {
		c, err := scope.NewCounter(10, 5)
		if err != nil || c == nil {
			t.Fatalf("NewCounter(10, 5) = %v, %v; want counter, nil", c != nil, err)
		}
		if diff := cmp.Diff([]int{10, 15, 20}, drainInts(c, 3)); diff != "" {
			t.Errorf("counter sequence mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("negative step and zero start", func(t *testing.T) {
		c, err := scope.NewCounter(0, -2)
		if err != nil || c == nil {
			t.Fatalf("NewCounter(0, -2) = %v, %v; want counter, nil", c != nil, err)
		}
		if diff := cmp.Diff([]int{0, -2, -4}, drainInts(c, 3)); diff != "" {
			t.Errorf("counter sequence mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("zero step rejected", func(t *testing.T) {
		if _, err := scope.NewCounter(0, 1); err != nil {
			t.Fatalf("NewCounter(0, 1): unexpected error: %v", err)
		}
		if _, err := scope.NewCounter(0, 0); !errors.Is(err, scope.ErrInvalidArgument) {
			t.Errorf("NewCounter(0, 0) error = %v; want ErrInvalidArgument", err)
		}
	})

	t.Run("counters are independent", func(t *testing.T) {
		a, err := scope.NewCounter(0, 1)
		if err != nil || a == nil {
			t.Fatalf("NewCounter(0, 1) = %v, %v; want counter, nil", a != nil, err)
		}
		b, err := scope.NewCounter(100, 1)
		if err != nil || b == nil {
			t.Fatalf("NewCounter(100, 1) = %v, %v; want counter, nil", b != nil, err)
		}
		got := []int{a(), a(), b(), a()}
		if diff := cmp.Diff([]int{0, 1, 100, 2}, got); diff != "" {
			t.Errorf("interleaved counters mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestPart2IDGenerator(t *testing.T) {
	t.Run("zero-padded sequence", func(t *testing.T) {
		g, err := scope.NewIDGenerator("inv", 4)
		if err != nil || g == nil {
			t.Fatalf("NewIDGenerator(inv, 4) = %v, %v; want generator, nil", g != nil, err)
		}
		got := []string{g(), g(), g()}
		want := []string{"inv-0001", "inv-0002", "inv-0003"}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("ids mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("width never truncates", func(t *testing.T) {
		g, err := scope.NewIDGenerator("x", 1)
		if err != nil || g == nil {
			t.Fatalf("NewIDGenerator(x, 1) = %v, %v; want generator, nil", g != nil, err)
		}
		var last string
		for range 9 {
			last = g()
		}
		if last != "x-9" {
			t.Fatalf("9th id = %q; want %q", last, "x-9")
		}
		if got := g(); got != "x-10" {
			t.Errorf("10th id = %q; want %q", got, "x-10")
		}
	})

	t.Run("invalid arguments", func(t *testing.T) {
		if _, err := scope.NewIDGenerator("a", 1); err != nil {
			t.Fatalf("NewIDGenerator(a, 1): unexpected error: %v", err)
		}
		cases := []struct {
			prefix string
			width  int
		}{
			{"", 3},
			{"a", 0},
			{"a", -1},
		}
		for _, tc := range cases {
			if _, err := scope.NewIDGenerator(tc.prefix, tc.width); !errors.Is(err, scope.ErrInvalidArgument) {
				t.Errorf("NewIDGenerator(%q, %d) error = %v; want ErrInvalidArgument", tc.prefix, tc.width, err)
			}
		}
	})

	t.Run("generators are independent", func(t *testing.T) {
		a, err := scope.NewIDGenerator("a", 2)
		if err != nil || a == nil {
			t.Fatalf("NewIDGenerator(a, 2) = %v, %v; want generator, nil", a != nil, err)
		}
		b, err := scope.NewIDGenerator("b", 2)
		if err != nil || b == nil {
			t.Fatalf("NewIDGenerator(b, 2) = %v, %v; want generator, nil", b != nil, err)
		}
		got := []string{a(), b(), a()}
		want := []string{"a-01", "b-01", "a-02"}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("ids mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestPart2ConcurrentIDs(t *testing.T) {
	g, err := scope.NewIDGenerator("job", 6)
	if err != nil || g == nil {
		t.Fatalf("NewIDGenerator(job, 6) = %v, %v; want generator, nil", g != nil, err)
	}

	const workers, perWorker = 50, 100
	var (
		mu   sync.Mutex
		seen = make(map[string]int)
		wg   sync.WaitGroup
	)
	for range workers {
		wg.Go(func() {
			for range perWorker {
				id := g()
				mu.Lock()
				seen[id]++
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	if len(seen) != workers*perWorker {
		t.Errorf("distinct ids = %d; want %d (every call unique)", len(seen), workers*perWorker)
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("id %q returned %d times; want 1", id, n)
			break
		}
	}
	if _, ok := seen["job-000001"]; !ok {
		t.Error(`id "job-000001" was never issued`)
	}
}

func TestPart2LateBinding(t *testing.T) {
	labels := []string{"save", "load", "quit"}

	t.Run("LoopCallbacks binds each label", func(t *testing.T) {
		fs := scope.LoopCallbacks(labels)
		if len(fs) != len(labels) {
			t.Fatalf("LoopCallbacks: %d callbacks; want %d", len(fs), len(labels))
		}
		for i, f := range fs {
			if f == nil {
				t.Fatalf("callback %d is nil", i)
			}
			want := "clicked " + labels[i]
			if got := f(); got != want {
				t.Errorf("callback %d() = %q; want %q", i, got, want)
			}
		}
	})

	t.Run("SharedCallbacks shares the last label", func(t *testing.T) {
		fs := scope.SharedCallbacks(labels)
		if len(fs) != len(labels) {
			t.Fatalf("SharedCallbacks: %d callbacks; want %d", len(fs), len(labels))
		}
		for i, f := range fs {
			if f == nil {
				t.Fatalf("callback %d is nil", i)
			}
			if got := f(); got != "clicked quit" {
				t.Errorf("callback %d() = %q; want %q", i, got, "clicked quit")
			}
		}
	})

	t.Run("LoopCallbacks is unaffected by later edits to labels", func(t *testing.T) {
		in := []string{"a", "b"}
		fs := scope.LoopCallbacks(in)
		if len(fs) != 2 || fs[0] == nil {
			t.Fatalf("LoopCallbacks(a, b): got %d callbacks; want 2", len(fs))
		}
		in[0] = "changed"
		if got := fs[0](); got != "clicked a" {
			t.Errorf("callback 0() = %q; want %q", got, "clicked a")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		if fs := scope.LoopCallbacks([]string{"x"}); len(fs) != 1 {
			t.Fatalf("LoopCallbacks([x]) len = %d; want 1", len(fs))
		}
		if fs := scope.LoopCallbacks(nil); len(fs) != 0 {
			t.Errorf("LoopCallbacks(nil) len = %d; want 0", len(fs))
		}
		if fs := scope.SharedCallbacks(nil); len(fs) != 0 {
			t.Errorf("SharedCallbacks(nil) len = %d; want 0", len(fs))
		}
	})
}

func TestPart2Adders(t *testing.T) {
	t.Run("each adder keeps its own offset", func(t *testing.T) {
		adders := scope.NewAdders([]int{1, 10, -3})
		if len(adders) != 3 {
			t.Fatalf("NewAdders: %d adders; want 3", len(adders))
		}
		want := []int{6, 15, 2}
		for i, add := range adders {
			if add == nil {
				t.Fatalf("adder %d is nil", i)
			}
			if got := add(5); got != want[i] {
				t.Errorf("adder %d(5) = %d; want %d", i, got, want[i])
			}
		}
	})

	t.Run("empty offsets", func(t *testing.T) {
		if got := scope.NewAdders([]int{7}); len(got) != 1 || got[0] == nil || got[0](1) != 8 {
			t.Fatal("NewAdders([7])[0](1) != 8")
		}
		if got := scope.NewAdders(nil); len(got) != 0 {
			t.Errorf("NewAdders(nil) len = %d; want 0", len(got))
		}
	})
}
