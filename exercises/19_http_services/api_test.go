package httpsvc_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	httpsvc "goref/exercises/19_http_services"
)

const validPut = `{"name":"bolt","quantity":2}`

func TestPart1CreateValidation(t *testing.T) {
	long64 := strings.Repeat("é", 64)
	long65 := strings.Repeat("a", 65)
	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
		wantField   string
	}{
		{"valid item", "application/json", `{"name":"bolt","quantity":1}`, http.StatusCreated, ""},
		{"charset parameter accepted", "application/json; charset=utf-8", `{"name":"washer","quantity":1}`, http.StatusCreated, ""},
		{"64 runes accepted", "application/json", `{"name":"` + long64 + `","quantity":0}`, http.StatusCreated, ""},
		{"65 runes rejected", "application/json", `{"name":"` + long65 + `","quantity":0}`, http.StatusUnprocessableEntity, "name"},
		{"blank name rejected", "application/json", `{"name":"   ","quantity":1}`, http.StatusUnprocessableEntity, "name"},
		{"negative quantity rejected", "application/json", `{"name":"nut","quantity":-1}`, http.StatusUnprocessableEntity, "quantity"},
		{"text/plain rejected", "text/plain", `{"name":"x","quantity":1}`, http.StatusUnsupportedMediaType, ""},
		{"missing content type rejected", "", `{"name":"x","quantity":1}`, http.StatusUnsupportedMediaType, ""},
		{"empty body rejected", "application/json", ``, http.StatusBadRequest, ""},
		{"malformed JSON rejected", "application/json", `{"name":`, http.StatusBadRequest, ""},
		{"unknown field rejected", "application/json", `{"name":"x","colour":"red"}`, http.StatusBadRequest, ""},
		{"trailing data rejected", "application/json", `{"name":"x","quantity":1}{}`, http.StatusBadRequest, ""},
		{"quantity as string rejected", "application/json", `{"name":"x","quantity":"3"}`, http.StatusBadRequest, ""},
		{"oversized body is 413", "application/json", `{"name":"` + strings.Repeat("a", httpsvc.MaxBodyBytes) + `"}`, http.StatusRequestEntityTooLarge, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := httpsvc.NewInventoryHandler()
			rec := do(t, h, http.MethodPost, "/items", tc.body, http.Header{"Content-Type": {tc.contentType}})
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, tc.wantStatus, rec.Body)
			}
			if tc.wantField != "" {
				if p := problemOf(t, rec); p.Errors[tc.wantField] == "" {
					t.Errorf("problem errors = %v, want an entry for %q", p.Errors, tc.wantField)
				}
			}
		})
	}
}

func TestPart1Create(t *testing.T) {
	t.Run("201 with location and etag", func(t *testing.T) {
		h := httpsvc.NewInventoryHandler()
		rec := do(t, h, http.MethodPost, "/items", `{"name":"bolt","quantity":5}`, jsonHeader())
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201", rec.Code)
		}
		if got := rec.Header().Get("Location"); got != "/items/1" {
			t.Errorf("Location = %q, want /items/1", got)
		}
		if got := rec.Header().Get("ETag"); got != `"1"` {
			t.Errorf("ETag = %q, want %q", got, `"1"`)
		}
		var item httpsvc.Item
		decodeBody(t, rec, &item)
		want := httpsvc.Item{ID: "1", Name: "bolt", Quantity: 5}
		if diff := cmp.Diff(want, item); diff != "" {
			t.Errorf("item mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("ids are sequential", func(t *testing.T) {
		h := httpsvc.NewInventoryHandler()
		first := createItem(t, h, "bolt", 1)
		second := createItem(t, h, "nut", 1)
		if first.ID != "1" || second.ID != "2" {
			t.Errorf("ids = %q, %q; want \"1\", \"2\"", first.ID, second.ID)
		}
	})
	t.Run("name is trimmed before storing", func(t *testing.T) {
		h := httpsvc.NewInventoryHandler()
		item := createItem(t, h, "  bolt  ", 1)
		if item.Name != "bolt" {
			t.Errorf("name = %q, want %q", item.Name, "bolt")
		}
	})
	t.Run("duplicate name conflicts case-insensitively", func(t *testing.T) {
		h := httpsvc.NewInventoryHandler()
		createItem(t, h, "Bolt", 1)
		rec := do(t, h, http.MethodPost, "/items", `{"name":"bolt","quantity":1}`, jsonHeader())
		if rec.Code != http.StatusConflict {
			t.Errorf("status = %d, want 409", rec.Code)
		}
	})
	t.Run("duplicate after trimming conflicts", func(t *testing.T) {
		h := httpsvc.NewInventoryHandler()
		createItem(t, h, "nut", 1)
		rec := do(t, h, http.MethodPost, "/items", `{"name":" NUT ","quantity":1}`, jsonHeader())
		if rec.Code != http.StatusConflict {
			t.Errorf("status = %d, want 409", rec.Code)
		}
	})
}

func TestPart1Read(t *testing.T) {
	h := httpsvc.NewInventoryHandler()
	createItem(t, h, "bolt", 5)
	t.Run("existing item returns body and etag", func(t *testing.T) {
		rec := do(t, h, http.MethodGet, "/items/1", "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := rec.Header().Get("ETag"); got != `"1"` {
			t.Errorf("ETag = %q, want %q", got, `"1"`)
		}
		var item httpsvc.Item
		decodeBody(t, rec, &item)
		if diff := cmp.Diff(httpsvc.Item{ID: "1", Name: "bolt", Quantity: 5}, item); diff != "" {
			t.Errorf("item mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("unknown id is a 404 problem", func(t *testing.T) {
		rec := do(t, h, http.MethodGet, "/items/99", "", nil)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
			t.Errorf("Content-Type = %q, want application/problem+json", ct)
		}
		if p := problemOf(t, rec); p.Status != http.StatusNotFound {
			t.Errorf("problem status = %d, want 404", p.Status)
		}
	})
	t.Run("non-numeric id is a 404", func(t *testing.T) {
		rec := do(t, h, http.MethodGet, "/items/abc", "", nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})
}

func TestPart1Update(t *testing.T) {
	tests := []struct {
		name       string
		extra      []string // names created before the request; "nut" becomes id 2
		target     string
		ifMatch    string
		body       string
		wantStatus int
		wantETag   string
	}{
		{"missing If-Match is 428", nil, "/items/1", "", validPut, http.StatusPreconditionRequired, ""},
		{"invalid body beats stale etag", nil, "/items/1", `"0"`, `{"name":"","quantity":2}`, http.StatusUnprocessableEntity, ""},
		{"stale etag is 412", nil, "/items/1", `"0"`, validPut, http.StatusPreconditionFailed, ""},
		{"weak etag never matches", nil, "/items/1", `W/"1"`, validPut, http.StatusPreconditionFailed, ""},
		{"unknown id beats stale etag", nil, "/items/99", `"0"`, validPut, http.StatusNotFound, ""},
		{"matching etag updates", nil, "/items/1", `"1"`, validPut, http.StatusOK, `"2"`},
		{"star matches any version", nil, "/items/1", "*", validPut, http.StatusOK, `"2"`},
		{"rename onto another item is 409", []string{"nut"}, "/items/1", `"1"`, `{"name":"NUT","quantity":2}`, http.StatusConflict, ""},
		{"keeping own name in other case is ok", nil, "/items/1", `"1"`, `{"name":"BOLT","quantity":2}`, http.StatusOK, `"2"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := httpsvc.NewInventoryHandler()
			createItem(t, h, "bolt", 1)
			for _, n := range tc.extra {
				createItem(t, h, n, 1)
			}
			hdr := jsonHeader()
			if tc.ifMatch != "" {
				hdr.Set("If-Match", tc.ifMatch)
			}
			rec := do(t, h, http.MethodPut, tc.target, tc.body, hdr)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, tc.wantStatus, rec.Body)
			}
			if tc.wantETag != "" {
				if got := rec.Header().Get("ETag"); got != tc.wantETag {
					t.Errorf("ETag = %q, want %q", got, tc.wantETag)
				}
			}
		})
	}

	t.Run("old etag is stale after a successful update", func(t *testing.T) {
		h := httpsvc.NewInventoryHandler()
		createItem(t, h, "bolt", 1)
		hdr := jsonHeader()
		hdr.Set("If-Match", `"1"`)
		if rec := do(t, h, http.MethodPut, "/items/1", validPut, hdr); rec.Code != http.StatusOK {
			t.Fatalf("first update status = %d, want 200", rec.Code)
		}
		rec := do(t, h, http.MethodPut, "/items/1", validPut, hdr)
		if rec.Code != http.StatusPreconditionFailed {
			t.Errorf("second update with old etag status = %d, want 412", rec.Code)
		}
	})
}

func TestPart1Delete(t *testing.T) {
	h := httpsvc.NewInventoryHandler()
	createItem(t, h, "bolt", 1)
	t.Run("existing item is deleted with 204", func(t *testing.T) {
		rec := do(t, h, http.MethodDelete, "/items/1", "", nil)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
		if after := do(t, h, http.MethodGet, "/items/1", "", nil); after.Code != http.StatusNotFound {
			t.Errorf("GET after delete = %d, want 404", after.Code)
		}
	})
	t.Run("deleting again is 404", func(t *testing.T) {
		rec := do(t, h, http.MethodDelete, "/items/1", "", nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})
}

func TestPart1MethodNotAllowed(t *testing.T) {
	h := httpsvc.NewInventoryHandler()
	createItem(t, h, "bolt", 1)
	rec := do(t, h, http.MethodPost, "/items/1", "", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("Allow = %q, want it to list GET", allow)
	}
}
