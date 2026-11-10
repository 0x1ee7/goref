package httpsvc_test

import (
	"errors"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	httpsvc "goref/exercises/19_http_services"
)

func TestPart3ParseListQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		want     httpsvc.ListQuery
		wantKeys []string // FieldErrors keys, sorted; nil means success
	}{
		{"defaults", "", httpsvc.ListQuery{Limit: httpsvc.DefaultLimit}, nil},
		{"max limit accepted", "limit=100", httpsvc.ListQuery{Limit: httpsvc.MaxLimit}, nil},
		{"search is trimmed", "q=%20%20Bolt%20", httpsvc.ListQuery{Limit: httpsvc.DefaultLimit, Search: "Bolt"}, nil},
		{"min_qty filter", "min_qty=3", httpsvc.ListQuery{Limit: httpsvc.DefaultLimit, MinQuantity: 3}, nil},
		{"limit zero rejected", "limit=0", httpsvc.ListQuery{}, []string{"limit"}},
		{"limit above max rejected", "limit=101", httpsvc.ListQuery{}, []string{"limit"}},
		{"limit not a number rejected", "limit=ten", httpsvc.ListQuery{}, []string{"limit"}},
		{"negative min_qty rejected", "min_qty=-1", httpsvc.ListQuery{}, []string{"min_qty"}},
		{"malformed cursor rejected", "cursor=!!!", httpsvc.ListQuery{}, []string{"cursor"}},
		{"all bad keys reported together", "limit=0&min_qty=x", httpsvc.ListQuery{}, []string{"limit", "min_qty"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v, _ := url.ParseQuery(tc.query)
			q, err := httpsvc.ParseListQuery(v)
			if tc.wantKeys == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if diff := cmp.Diff(tc.want, q); diff != "" {
					t.Errorf("ListQuery (-want +got):\n%s", diff)
				}
				return
			}
			var fe httpsvc.FieldErrors
			if !errors.As(err, &fe) {
				t.Fatalf("err = %v, want FieldErrors", err)
			}
			if diff := cmp.Diff(tc.wantKeys, slices.Sorted(maps.Keys(fe))); diff != "" {
				t.Errorf("error keys (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPart3ValuesRoundTrip(t *testing.T) {
	q := httpsvc.ListQuery{Limit: 7, Search: "bolt", MinQuantity: 2}
	got, err := httpsvc.ParseListQuery(q.Values())
	if err != nil {
		t.Fatalf("ParseListQuery(q.Values()): %v", err)
	}
	if diff := cmp.Diff(q, got); diff != "" {
		t.Errorf("round trip (-want +got):\n%s", diff)
	}
}

// listItems GETs /items with the raw query and decodes the page.
func listItems(t *testing.T, h http.Handler, query string) httpsvc.ItemPage {
	t.Helper()
	rec := do(t, h, http.MethodGet, "/items?"+query, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /items?%s: status %d, body %s", query, rec.Code, rec.Body)
	}
	var page httpsvc.ItemPage
	decodeBody(t, rec, &page)
	return page
}

func TestPart3Pagination(t *testing.T) {
	t.Run("walks every page in creation order", func(t *testing.T) {
		h := httpsvc.NewInventoryHandler()
		for _, n := range []string{"item-1", "item-2", "item-3", "item-4", "item-5"} {
			createItem(t, h, n, 1)
		}
		var names []string
		var sizes []int
		cursor := ""
		for range 10 {
			query := "limit=2"
			if cursor != "" {
				query += "&cursor=" + cursor
			}
			page := listItems(t, h, query)
			sizes = append(sizes, len(page.Items))
			for _, it := range page.Items {
				names = append(names, it.Name)
			}
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
		}
		if diff := cmp.Diff([]string{"item-1", "item-2", "item-3", "item-4", "item-5"}, names); diff != "" {
			t.Errorf("names (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff([]int{2, 2, 1}, sizes); diff != "" {
			t.Errorf("page sizes (-want +got):\n%s", diff)
		}
	})
	t.Run("exact multiple ends without a cursor", func(t *testing.T) {
		h := httpsvc.NewInventoryHandler()
		for _, n := range []string{"a", "b", "c", "d"} {
			createItem(t, h, n, 1)
		}
		first := listItems(t, h, "limit=2")
		if first.NextCursor == "" {
			t.Fatal("first page has no next_cursor")
		}
		last := listItems(t, h, "limit=2&cursor="+first.NextCursor)
		if len(last.Items) != 2 || last.NextCursor != "" {
			t.Errorf("last page = %d items, cursor %q; want 2 items and no cursor", len(last.Items), last.NextCursor)
		}
	})
	t.Run("empty listing encodes items as []", func(t *testing.T) {
		h := httpsvc.NewInventoryHandler()
		rec := do(t, h, http.MethodGet, "/items", "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"items":[]`) {
			t.Errorf("body = %s, want \"items\":[]", rec.Body)
		}
	})
	t.Run("search is a case-insensitive substring", func(t *testing.T) {
		h := httpsvc.NewInventoryHandler()
		createItem(t, h, "Bolt", 1)
		createItem(t, h, "nut", 1)
		createItem(t, h, "bolt-pack", 1)
		page := listItems(t, h, "q=BOLT")
		var names []string
		for _, it := range page.Items {
			names = append(names, it.Name)
		}
		if diff := cmp.Diff([]string{"Bolt", "bolt-pack"}, names); diff != "" {
			t.Errorf("names (-want +got):\n%s", diff)
		}
	})
	t.Run("min_qty filters out smaller stock", func(t *testing.T) {
		h := httpsvc.NewInventoryHandler()
		createItem(t, h, "bolt", 5)
		createItem(t, h, "nut", 0)
		createItem(t, h, "washer", 10)
		page := listItems(t, h, "min_qty=5")
		var names []string
		for _, it := range page.Items {
			names = append(names, it.Name)
		}
		if diff := cmp.Diff([]string{"bolt", "washer"}, names); diff != "" {
			t.Errorf("names (-want +got):\n%s", diff)
		}
	})
	t.Run("cursor survives deletion of its item", func(t *testing.T) {
		h := httpsvc.NewInventoryHandler()
		for _, n := range []string{"item-1", "item-2", "item-3"} {
			createItem(t, h, n, 1)
		}
		first := listItems(t, h, "limit=1")
		if del := do(t, h, http.MethodDelete, "/items/1", "", nil); del.Code != http.StatusNoContent {
			t.Fatalf("delete status = %d, want 204", del.Code)
		}
		second := listItems(t, h, "limit=1&cursor="+first.NextCursor)
		if len(second.Items) != 1 || second.Items[0].Name != "item-2" {
			t.Errorf("page after delete = %v, want item-2", second.Items)
		}
	})
}

func TestPart3InvalidQueries(t *testing.T) {
	h := httpsvc.NewInventoryHandler()
	tests := []struct {
		name  string
		query string
		key   string
	}{
		{"malformed cursor", "cursor=%21%21", "cursor"},
		{"limit out of range", "limit=500", "limit"},
		{"negative min_qty", "min_qty=-2", "min_qty"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, h, http.MethodGet, "/items?"+tc.query, "", nil)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if p := problemOf(t, rec); p.Errors[tc.key] == "" {
				t.Errorf("problem errors = %v, want key %q", p.Errors, tc.key)
			}
		})
	}
}
