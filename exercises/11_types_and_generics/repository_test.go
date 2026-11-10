package typed_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/go-cmp/cmp"

	"goref/exercises/11_types_and_generics"
)

// user is a value entity: Key has a value receiver.
type user struct {
	ID   typed.UserID
	Name string
}

func (u user) Key() typed.UserID { return u.ID }

// order is a pointer entity: Key is declared on *order, so T = *order.
type order struct {
	ID    typed.OrderID
	Total int
}

func (o *order) Key() typed.OrderID { return o.ID }

func ids(users []user) []typed.UserID {
	out := make([]typed.UserID, 0, len(users))
	for _, u := range users {
		out = append(out, u.ID)
	}
	return out
}

func TestPart5CreateAndGet(t *testing.T) {
	var repo typed.Repository[typed.UserID, user]
	if err := repo.Create(user{ID: 1, Name: "ada"}); err != nil {
		t.Fatalf("Create(1): %v", err)
	}
	got, err := repo.Get(1)
	if err != nil {
		t.Fatalf("Get(1): %v", err)
	}
	if diff := cmp.Diff(user{ID: 1, Name: "ada"}, got); diff != "" {
		t.Errorf("Get(1) mismatch (-want +got):\n%s", diff)
	}

	missing, err := repo.Get(2)
	if !errors.Is(err, typed.ErrNotFound) {
		t.Errorf("Get(2) error = %v, want ErrNotFound", err)
	}
	if missing != (user{}) {
		t.Errorf("Get(2) value = %+v, want zero value", missing)
	}
}

func TestPart5CreateDuplicate(t *testing.T) {
	var repo typed.Repository[typed.UserID, user]
	if err := repo.Create(user{ID: 1, Name: "ada"}); err != nil {
		t.Fatalf("Create(1): %v", err)
	}
	if err := repo.Create(user{ID: 1, Name: "impostor"}); !errors.Is(err, typed.ErrExists) {
		t.Errorf("duplicate Create error = %v, want ErrExists", err)
	}
	got, err := repo.Get(1)
	if err != nil || got.Name != "ada" {
		t.Errorf("after rejected duplicate Get(1) = (%+v, %v), want name ada", got, err)
	}
}

func TestPart5ReplaceAndDelete(t *testing.T) {
	var repo typed.Repository[typed.UserID, user]
	if err := repo.Replace(user{ID: 2, Name: "x"}); !errors.Is(err, typed.ErrNotFound) {
		t.Errorf("Replace of absent key error = %v, want ErrNotFound", err)
	}
	for _, id := range []typed.UserID{1, 2, 3} {
		if err := repo.Create(user{ID: id, Name: "v1"}); err != nil {
			t.Fatalf("Create(%d): %v", id, err)
		}
	}
	if err := repo.Replace(user{ID: 2, Name: "v2"}); err != nil {
		t.Fatalf("Replace(2): %v", err)
	}
	if got, err := repo.Get(2); err != nil || got.Name != "v2" {
		t.Errorf("Get(2) after Replace = (%+v, %v), want name v2", got, err)
	}

	if err := repo.Delete(2); err != nil {
		t.Fatalf("Delete(2): %v", err)
	}
	if err := repo.Delete(2); !errors.Is(err, typed.ErrNotFound) {
		t.Errorf("second Delete(2) error = %v, want ErrNotFound", err)
	}
	if _, err := repo.Get(2); !errors.Is(err, typed.ErrNotFound) {
		t.Errorf("Get(2) after Delete error = %v, want ErrNotFound", err)
	}
	if repo.Len() != 2 {
		t.Errorf("Len() = %d, want 2", repo.Len())
	}
}

func TestPart5FindKeepsInsertionOrder(t *testing.T) {
	var repo typed.Repository[typed.UserID, user]
	for _, id := range []typed.UserID{3, 1, 2} {
		if err := repo.Create(user{ID: id, Name: "n"}); err != nil {
			t.Fatalf("Create(%d): %v", id, err)
		}
	}
	// Replacing keeps the entity's position.
	if err := repo.Replace(user{ID: 1, Name: "renamed"}); err != nil {
		t.Fatalf("Replace(1): %v", err)
	}

	all := repo.Find(func(user) bool { return true })
	if diff := cmp.Diff([]typed.UserID{3, 1, 2}, ids(all)); diff != "" {
		t.Errorf("Find(all) order mismatch (-want +got):\n%s", diff)
	}
	big := repo.Find(func(u user) bool { return u.ID > 1 })
	if diff := cmp.Diff([]typed.UserID{3, 2}, ids(big)); diff != "" {
		t.Errorf("Find(ID>1) mismatch (-want +got):\n%s", diff)
	}
	none := repo.Find(func(user) bool { return false })
	if none == nil || len(none) != 0 {
		t.Errorf("Find(no match) = %#v, want non-nil empty slice", none)
	}
}

func TestPart5ReturnedValuesAreCopies(t *testing.T) {
	var repo typed.Repository[typed.UserID, user]
	if err := repo.Create(user{ID: 1, Name: "ada"}); err != nil {
		t.Fatalf("Create(1): %v", err)
	}
	u, err := repo.Get(1)
	if err != nil || u.Name != "ada" {
		t.Fatalf("Get(1) = (%+v, %v), want name ada", u, err)
	}
	u.Name = "changed"
	again, _ := repo.Get(1)
	if again.Name != "ada" {
		t.Errorf("stored name after mutating returned copy = %q, want ada", again.Name)
	}
}

func TestPart5PointerEntities(t *testing.T) {
	var repo typed.Repository[typed.OrderID, *order]
	o := &order{ID: 9, Total: 100}
	if err := repo.Create(o); err != nil {
		t.Fatalf("Create(order 9): %v", err)
	}
	got, err := repo.Get(9)
	if err != nil || got == nil || got.Total != 100 {
		t.Fatalf("Get(9) = (%+v, %v), want Total 100", got, err)
	}
	// A pointer entity is shared: the caller's mutation is visible to the store.
	o.Total = 250
	if again, _ := repo.Get(9); again.Total != 250 {
		t.Errorf("Total after caller mutation = %d, want 250", again.Total)
	}
	if err := repo.Create(&order{ID: 9}); !errors.Is(err, typed.ErrExists) {
		t.Errorf("duplicate pointer Create error = %v, want ErrExists", err)
	}
}

func TestPart5ConcurrentCreate(t *testing.T) {
	t.Run("same key has one winner", func(t *testing.T) {
		var repo typed.Repository[typed.UserID, user]
		var wins, exists atomic.Int64
		var wg sync.WaitGroup
		for range 32 {
			wg.Go(func() {
				switch err := repo.Create(user{ID: 7, Name: "dup"}); {
				case err == nil:
					wins.Add(1)
				case errors.Is(err, typed.ErrExists):
					exists.Add(1)
				}
			})
		}
		wg.Wait()
		if wins.Load() != 1 || exists.Load() != 31 {
			t.Errorf("wins=%d exists=%d, want 1 and 31", wins.Load(), exists.Load())
		}
	})

	t.Run("distinct keys all land", func(t *testing.T) {
		var repo typed.Repository[typed.UserID, user]
		var wg sync.WaitGroup
		for i := range 100 {
			wg.Go(func() {
				if err := repo.Create(user{ID: typed.UserID(i + 1)}); err != nil {
					t.Errorf("Create(%d): %v", i+1, err)
				}
			})
		}
		wg.Wait()
		if repo.Len() != 100 {
			t.Errorf("Len() = %d, want 100", repo.Len())
		}
		if got := repo.Find(func(user) bool { return true }); len(got) != 100 {
			t.Errorf("Find(all) returned %d entities, want 100", len(got))
		}
	})
}
