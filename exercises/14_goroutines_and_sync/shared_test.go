package goroutines_test

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/go-cmp/cmp"
	"goref/exercises/14_goroutines_and_sync"
)

func TestPart3CounterAdd(t *testing.T) {
	var c goroutines.Counter
	steps := []struct {
		name  string
		delta int64
		want  int64
	}{
		{name: "first add", delta: 5, want: 5},
		{name: "negative delta", delta: -2, want: 3},
		{name: "zero delta keeps value", delta: 0, want: 3},
	}
	for _, s := range steps {
		t.Run(s.name, func(t *testing.T) {
			if got := c.Add("hits", s.delta); got != s.want {
				t.Errorf("Add(hits, %d) = %d, want %d", s.delta, got, s.want)
			}
		})
	}
	if got := c.Get("hits"); got != 3 {
		t.Errorf("Get(hits) = %d, want 3", got)
	}
	if got := c.Get("never-set"); got != 0 {
		t.Errorf("Get(unknown) = %d, want 0", got)
	}
}

func TestPart3CounterConcurrentAdd(t *testing.T) {
	var c goroutines.Counter
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			for range 100 {
				c.Add("requests", 1)
			}
		})
	}
	callWithin(t, "concurrent adds", wg.Wait)
	if got := c.Get("requests"); got != 5000 {
		t.Errorf("Get(requests) = %d after 5000 concurrent adds, want 5000", got)
	}
}

func TestPart3CounterSnapshotIsCopy(t *testing.T) {
	var c goroutines.Counter
	c.Add("a", 1)
	c.Add("b", 2)
	snap := c.Snapshot()
	want := map[string]int64{"a": 1, "b": 2}
	if diff := cmp.Diff(want, snap); diff != "" {
		t.Fatalf("Snapshot() mismatch (-want +got):\n%s", diff)
	}
	snap["a"] = 999
	delete(snap, "b")
	if got := c.Get("a"); got != 1 {
		t.Errorf("Get(a) = %d after mutating snapshot, want 1 (snapshot must be a copy)", got)
	}
	if got := c.Get("b"); got != 2 {
		t.Errorf("Get(b) = %d after mutating snapshot, want 2", got)
	}
}

func TestPart3NewAccountValidation(t *testing.T) {
	t.Run("negative balance rejected", func(t *testing.T) {
		_, err := goroutines.NewAccount("acc", -1)
		if !errors.Is(err, goroutines.ErrInvalidAmount) {
			t.Errorf("NewAccount(-1) error = %v, want ErrInvalidAmount", err)
		}
	})
	t.Run("opening balance and id", func(t *testing.T) {
		a, err := goroutines.NewAccount("acc-1", 100)
		if err != nil {
			t.Fatalf("NewAccount() error = %v", err)
		}
		if a.ID() != "acc-1" {
			t.Errorf("ID() = %q, want acc-1", a.ID())
		}
		if a.Balance() != 100 {
			t.Errorf("Balance() = %d, want 100", a.Balance())
		}
	})
}

func TestPart3AccountDepositWithdraw(t *testing.T) {
	a, err := goroutines.NewAccount("acc", 50)
	if err != nil {
		t.Fatalf("NewAccount() error = %v", err)
	}
	tests := []struct {
		name    string
		op      func() error
		wantErr error
		want    int64
	}{
		{name: "deposit 25", op: func() error { return a.Deposit(25) }, want: 75},
		{name: "deposit zero rejected", op: func() error { return a.Deposit(0) }, wantErr: goroutines.ErrInvalidAmount, want: 75},
		{name: "deposit negative rejected", op: func() error { return a.Deposit(-5) }, wantErr: goroutines.ErrInvalidAmount, want: 75},
		{name: "withdraw 30", op: func() error { return a.Withdraw(30) }, want: 45},
		{name: "withdraw more than balance", op: func() error { return a.Withdraw(46) }, wantErr: goroutines.ErrInsufficientFunds, want: 45},
		{name: "withdraw exact balance", op: func() error { return a.Withdraw(45) }, want: 0},
		{name: "withdraw zero rejected", op: func() error { return a.Withdraw(0) }, wantErr: goroutines.ErrInvalidAmount, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.op()
			if tt.wantErr == nil && err != nil {
				t.Fatalf("unexpected error = %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if got := a.Balance(); got != tt.want {
				t.Errorf("Balance() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPart3TransferMovesMoney(t *testing.T) {
	src, _ := goroutines.NewAccount("src", 100)
	dst, _ := goroutines.NewAccount("dst", 10)
	if err := goroutines.Transfer(src, dst, 40); err != nil {
		t.Fatalf("Transfer() error = %v", err)
	}
	if src.Balance() != 60 || dst.Balance() != 50 {
		t.Errorf("balances src=%d dst=%d, want 60 and 50", src.Balance(), dst.Balance())
	}
}

func TestPart3TransferRejections(t *testing.T) {
	tests := []struct {
		name    string
		amount  int64
		same    bool
		srcBal  int64
		wantErr error
	}{
		{name: "zero amount", amount: 0, srcBal: 100, wantErr: goroutines.ErrInvalidAmount},
		{name: "negative amount", amount: -1, srcBal: 100, wantErr: goroutines.ErrInvalidAmount},
		{name: "same account", amount: 10, same: true, srcBal: 100, wantErr: goroutines.ErrSameAccount},
		{name: "insufficient funds", amount: 101, srcBal: 100, wantErr: goroutines.ErrInsufficientFunds},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, _ := goroutines.NewAccount("src", tt.srcBal)
			dst, _ := goroutines.NewAccount("dst", 5)
			to := dst
			if tt.same {
				to = src
			}
			err := goroutines.Transfer(src, to, tt.amount)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Transfer() error = %v, want %v", err, tt.wantErr)
			}
			if src.Balance() != tt.srcBal {
				t.Errorf("src balance = %d after rejected transfer, want %d", src.Balance(), tt.srcBal)
			}
			if !tt.same && dst.Balance() != 5 {
				t.Errorf("dst balance = %d after rejected transfer, want 5", dst.Balance())
			}
		})
	}
}

func TestPart3TransferOppositeDirectionsDoNotDeadlock(t *testing.T) {
	a, _ := goroutines.NewAccount("a", 1000)
	b, _ := goroutines.NewAccount("b", 1000)
	var wg sync.WaitGroup
	for range 200 {
		wg.Go(func() {
			_ = goroutines.Transfer(a, b, 1)
		})
		wg.Go(func() {
			_ = goroutines.Transfer(b, a, 1)
		})
	}
	callWithin(t, "opposing transfers", wg.Wait)
	if total := a.Balance() + b.Balance(); total != 2000 {
		t.Errorf("total money = %d, want 2000", total)
	}
}

func TestPart3TransferConservesMoney(t *testing.T) {
	names := []string{"w", "x", "y", "z"}
	accounts := make([]*goroutines.Account, len(names))
	for i, n := range names {
		accounts[i], _ = goroutines.NewAccount(n, 500)
	}
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Go(func() {
			for i := range 300 {
				src := accounts[(g+i)%len(accounts)]
				dst := accounts[(g+2*i+1)%len(accounts)]
				_ = goroutines.Transfer(src, dst, int64(1+i%7))
			}
		})
	}
	callWithin(t, "random-ish transfers", wg.Wait)
	var total int64
	for _, a := range accounts {
		if a.Balance() < 0 {
			t.Errorf("account %s balance = %d, want >= 0", a.ID(), a.Balance())
		}
		total += a.Balance()
	}
	if total != 2000 {
		t.Errorf("total money = %d after transfers, want 2000", total)
	}
}

func TestPart3Registry(t *testing.T) {
	var r goroutines.Registry
	r.Register("billing", "10.0.0.2:8080")
	r.Register("auth", "10.0.0.1:8080")
	r.Register("billing", "10.0.0.3:8080") // replaces

	if addr, ok := r.Lookup("billing"); !ok || addr != "10.0.0.3:8080" {
		t.Errorf("Lookup(billing) = %q, %v; want replaced address, true", addr, ok)
	}
	if addr, ok := r.Lookup("auth"); !ok || addr != "10.0.0.1:8080" {
		t.Errorf("Lookup(auth) = %q, %v; want 10.0.0.1:8080, true", addr, ok)
	}
	if addr, ok := r.Lookup("missing"); ok || addr != "" {
		t.Errorf("Lookup(missing) = %q, %v; want empty, false", addr, ok)
	}
	if diff := cmp.Diff([]string{"auth", "billing"}, r.Names()); diff != "" {
		t.Errorf("Names() mismatch (-want +got):\n%s", diff)
	}
}

func TestPart3RegistryConcurrentUse(t *testing.T) {
	var r goroutines.Registry
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() {
			r.Register(fmt.Sprintf("svc-%02d", i), "addr")
		})
		wg.Go(func() {
			r.Lookup("svc-00")
			r.Names()
		})
	}
	callWithin(t, "registry writers and readers", wg.Wait)
	if n := len(r.Names()); n != 20 {
		t.Errorf("len(Names()) = %d, want 20", n)
	}
}

func TestPart3PermanentOnce(t *testing.T) {
	t.Run("value loaded once for concurrent callers", func(t *testing.T) {
		var loads atomic.Int64
		get := goroutines.PermanentOnce(func() (int, error) {
			loads.Add(1)
			return 42, nil
		})
		var wg sync.WaitGroup
		results := make(chan int, 20)
		for range 20 {
			wg.Go(func() {
				v, err := get()
				if err != nil {
					t.Errorf("get() error = %v", err)
				}
				results <- v
			})
		}
		callWithin(t, "concurrent get()", wg.Wait)
		close(results)
		for v := range results {
			if v != 42 {
				t.Errorf("get() = %d, want 42", v)
			}
		}
		if n := loads.Load(); n != 1 {
			t.Errorf("load called %d times, want 1", n)
		}
	})
	t.Run("error is cached forever", func(t *testing.T) {
		var loads atomic.Int64
		get := goroutines.PermanentOnce(func() (string, error) {
			loads.Add(1)
			return "", errBoom
		})
		for i := range 3 {
			_, err := get()
			if !errors.Is(err, errBoom) {
				t.Fatalf("call %d error = %v, want errBoom", i, err)
			}
		}
		if n := loads.Load(); n != 1 {
			t.Errorf("load called %d times, want 1 (errors are cached too)", n)
		}
	})
}

func TestPart3LazyCachesSuccessOnly(t *testing.T) {
	t.Run("success is cached", func(t *testing.T) {
		var loads atomic.Int64
		l := goroutines.NewLazy(func() (string, error) {
			loads.Add(1)
			return "config", nil
		})
		for range 3 {
			v, err := l.Get()
			if err != nil || v != "config" {
				t.Fatalf("Get() = %q, %v; want config, nil", v, err)
			}
		}
		if n := loads.Load(); n != 1 {
			t.Errorf("load called %d times, want 1", n)
		}
	})
	t.Run("failure is not cached", func(t *testing.T) {
		var attempts int
		l := goroutines.NewLazy(func() (int, error) {
			attempts++
			if attempts == 1 {
				return 0, errBoom
			}
			return 7, nil
		})
		if _, err := l.Get(); !errors.Is(err, errBoom) {
			t.Fatalf("first Get() error = %v, want errBoom", err)
		}
		v, err := l.Get()
		if err != nil || v != 7 {
			t.Errorf("second Get() = %d, %v; want 7, nil (failure must not be cached)", v, err)
		}
		if attempts != 2 {
			t.Errorf("load attempts = %d, want 2", attempts)
		}
	})
	t.Run("concurrent first Get loads once", func(t *testing.T) {
		var loads atomic.Int64
		l := goroutines.NewLazy(func() (int, error) {
			loads.Add(1)
			return 1, nil
		})
		var wg sync.WaitGroup
		for range 16 {
			wg.Go(func() {
				if _, err := l.Get(); err != nil {
					t.Errorf("Get() error = %v", err)
				}
			})
		}
		callWithin(t, "concurrent Get()", wg.Wait)
		if n := loads.Load(); n != 1 {
			t.Errorf("load called %d times under contention, want 1", n)
		}
	})
}
