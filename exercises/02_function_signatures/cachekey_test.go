package signatures_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"goref/exercises/02_function_signatures"
)

type UserID int64

func TestPart6Key(t *testing.T) {
	ts := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	tests := []struct {
		name  string
		ns    string
		parts []any
		want  string
	}{
		{name: "scalars", ns: "users", parts: []any{42, true, "x"}, want: "users:42:true:x"},
		{name: "namespace only", ns: "ns", want: "ns"},
		{name: "named integer type", ns: "users", parts: []any{UserID(7)}, want: "users:7"},
		{name: "unsigned and negative", ns: "n", parts: []any{uint8(255), -3}, want: "n:255:-3"},
		{
			name:  "time is rendered in UTC",
			ns:    "ns",
			parts: []any{ts.In(time.FixedZone("CEST", 2*3600))},
			want:  "ns:2026-01-02T03:04:05Z",
		},
		{
			name:  "time keeps nanoseconds",
			ns:    "ns",
			parts: []any{ts.Add(123 * time.Millisecond)},
			want:  "ns:2026-01-02T03:04:05.123Z",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := signatures.Key(tc.ns, tc.parts...)
			if err != nil {
				t.Fatalf("Key unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("Key = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPart6KeyErrors(t *testing.T) {
	tests := []struct {
		name    string
		ns      string
		parts   []any
		wantErr error
	}{
		{name: "empty namespace", ns: "", parts: []any{1}, wantErr: signatures.ErrInvalidNamespace},
		{name: "namespace with colon", ns: "a:b", parts: []any{1}, wantErr: signatures.ErrInvalidNamespace},
		{name: "nil part", ns: "ns", parts: []any{nil}, wantErr: signatures.ErrUnsupportedKeyPart},
		{name: "float part", ns: "ns", parts: []any{1.5}, wantErr: signatures.ErrUnsupportedKeyPart},
		{name: "slice part", ns: "ns", parts: []any{[]string{"a"}}, wantErr: signatures.ErrUnsupportedKeyPart},
		{name: "struct part", ns: "ns", parts: []any{struct{ A int }{1}}, wantErr: signatures.ErrUnsupportedKeyPart},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := signatures.Key(tc.ns, tc.parts...)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Key error = %v, want errors.Is %v", err, tc.wantErr)
			}
		})
	}
}

func TestPart6KeyCollisions(t *testing.T) {
	t.Run("separator inside a part cannot forge a boundary", func(t *testing.T) {
		k1, err1 := signatures.Key("ns", "a:b", "c")
		k2, err2 := signatures.Key("ns", "a", "b:c")
		if err1 != nil || err2 != nil {
			t.Fatalf("Key errors: %v, %v", err1, err2)
		}
		if k1 == k2 {
			t.Errorf("distinct part lists produced the same key %q", k1)
		}
	})
	t.Run("same arguments produce the same key", func(t *testing.T) {
		for range 2 {
			got, err := signatures.Key("ns", 1, "x")
			if err != nil {
				t.Fatalf("Key unexpected error: %v", err)
			}
			if got != "ns:1:x" {
				t.Fatalf("Key = %q, want %q", got, "ns:1:x")
			}
		}
	})
	t.Run("different namespaces produce different keys", func(t *testing.T) {
		a, _ := signatures.Key("a", 1)
		b, _ := signatures.Key("b", 1)
		if a != "a:1" || b != "b:1" {
			t.Errorf("Key(a,1)=%q Key(b,1)=%q, want a:1 and b:1", a, b)
		}
	})
}

func TestPart6CacheGetSet(t *testing.T) {
	var c signatures.Cache[int]
	if _, ok := c.Get("missing"); ok {
		t.Fatal("Get on zero Cache reported a hit")
	}
	c.Set("k", 5)
	if v, ok := c.Get("k"); v != 5 || !ok {
		t.Fatalf("Get(k) = (%d, %v), want (5, true)", v, ok)
	}
	c.Set("k", 6)
	if v, ok := c.Get("k"); v != 6 || !ok {
		t.Errorf("after overwrite Get(k) = (%d, %v), want (6, true)", v, ok)
	}
}

func TestPart6RememberLoadsOnce(t *testing.T) {
	var c signatures.Cache[string]
	calls := 0
	load := func(context.Context) (string, error) {
		calls++
		return "v", nil
	}
	v1, err := c.Remember(t.Context(), "k", load)
	if err != nil || v1 != "v" {
		t.Fatalf("first Remember = (%q, %v), want (\"v\", nil)", v1, err)
	}
	v2, err := c.Remember(t.Context(), "k", load)
	if err != nil || v2 != "v" {
		t.Fatalf("second Remember = (%q, %v), want (\"v\", nil)", v2, err)
	}
	if calls != 1 {
		t.Errorf("load called %d times, want 1", calls)
	}
}

func TestPart6RememberDoesNotCacheErrors(t *testing.T) {
	var c signatures.Cache[string]
	boom := errors.New("upstream timeout")
	calls := 0
	_, err := c.Remember(t.Context(), "k", func(context.Context) (string, error) {
		calls++
		return "", boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("first Remember error = %v, want errors.Is boom", err)
	}
	v, err := c.Remember(t.Context(), "k", func(context.Context) (string, error) {
		calls++
		return "ok", nil
	})
	if err != nil || v != "ok" {
		t.Fatalf("second Remember = (%q, %v), want (\"ok\", nil)", v, err)
	}
	if calls != 2 {
		t.Errorf("load called %d times, want 2 (error must not be cached)", calls)
	}
}

func TestPart6RememberPassesContext(t *testing.T) {
	type ctxKey struct{}
	ctx := context.WithValue(t.Context(), ctxKey{}, "req-42")
	var c signatures.Cache[string]
	v, err := c.Remember(ctx, "k", func(ctx context.Context) (string, error) {
		s, _ := ctx.Value(ctxKey{}).(string)
		return s, nil
	})
	if err != nil {
		t.Fatalf("Remember unexpected error: %v", err)
	}
	if v != "req-42" {
		t.Errorf("load saw context value %q, want %q", v, "req-42")
	}
}

func TestPart6CacheConcurrentSet(t *testing.T) {
	const n = 50
	var c signatures.Cache[int]
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			c.Set(fmt.Sprint(i), i)
		})
	}
	wg.Wait()

	var missing []string
	for i := range n {
		if v, ok := c.Get(fmt.Sprint(i)); !ok || v != i {
			missing = append(missing, fmt.Sprint(i))
		}
	}
	if diff := cmp.Diff([]string(nil), missing); diff != "" {
		t.Errorf("keys missing or wrong after concurrent Set (-want +got):\n%s", diff)
	}
}
