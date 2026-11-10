// Package httpsvc is the 19_http_services exercise: an inventory REST API,
// its middleware, cursor pagination, a retrying HTTP client and an NDJSON
// event stream. See README.md for the full spec.
package httpsvc

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

// MaxBodyBytes caps the size of request bodies accepted by the API.
const MaxBodyBytes = 1 << 20

// maxNameRunes is the longest accepted item name, in runes.
const maxNameRunes = 64

// Item is an inventory item as exchanged over the wire.
type Item struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

// ItemInput is the request body for creating or replacing an item.
type ItemInput struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

// Problem is an RFC 9457 problem details document.
type Problem struct {
	Type   string            `json:"type"`
	Title  string            `json:"title"`
	Status int               `json:"status"`
	Detail string            `json:"detail,omitempty"`
	Errors map[string]string `json:"errors,omitempty"`
}

// FieldErrors maps a field name to a validation reason. It is the error
// returned by validation helpers.
type FieldErrors map[string]string

// Error implements error.
func (f FieldErrors) Error() string {
	return "validation failed"
}

var (
	errNotFound     = errors.New("item not found")
	errDuplicate    = errors.New("duplicate item name")
	errPrecondition = errors.New("etag does not match")
)

// record is a stored item. seq is the creation order and the ID source.
type record struct {
	seq     int64
	item    Item
	version int64
}

// inventory is the in-memory store. Records stay in creation order.
type inventory struct {
	mu    sync.Mutex
	next  int64
	order []record
	index map[string]int // ID -> position in order
}

func newInventory() *inventory {
	return &inventory{index: map[string]int{}}
}

// nameTaken reports whether another item already uses name. Callers hold mu.
func (inv *inventory) nameTaken(name, exceptID string) bool {
	for _, r := range inv.order {
		if r.item.ID != exceptID && strings.EqualFold(r.item.Name, name) {
			return true
		}
	}
	return false
}

func (inv *inventory) create(in ItemInput) (Item, int64, error) {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	if inv.nameTaken(in.Name, "") {
		return Item{}, 0, errDuplicate
	}
	inv.next++
	r := record{
		seq:     inv.next,
		item:    Item{ID: strconv.FormatInt(inv.next, 10), Name: in.Name, Quantity: in.Quantity},
		version: 1,
	}
	inv.index[r.item.ID] = len(inv.order)
	inv.order = append(inv.order, r)
	return r.item, r.version, nil
}

func (inv *inventory) get(id string) (Item, int64, error) {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	i, ok := inv.index[id]
	if !ok {
		return Item{}, 0, errNotFound
	}
	return inv.order[i].item, inv.order[i].version, nil
}

// update replaces an item. match decides whether the caller's precondition
// holds for the current version; it runs after the existence check.
func (inv *inventory) update(id string, in ItemInput, match func(version int64) bool) (Item, int64, error) {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	i, ok := inv.index[id]
	if !ok {
		return Item{}, 0, errNotFound
	}
	r := &inv.order[i]
	if !match(r.version) {
		return Item{}, 0, errPrecondition
	}
	if inv.nameTaken(in.Name, id) {
		return Item{}, 0, errDuplicate
	}
	r.item.Name = in.Name
	r.item.Quantity = in.Quantity
	r.version++
	return r.item, r.version, nil
}

func (inv *inventory) remove(id string) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	i, ok := inv.index[id]
	if !ok {
		return errNotFound
	}
	inv.order = append(inv.order[:i], inv.order[i+1:]...)
	delete(inv.index, id)
	for j := i; j < len(inv.order); j++ {
		inv.index[inv.order[j].item.ID] = j
	}
	return nil
}

// NewInventoryHandler returns the REST API for items, mounted at /items.
func NewInventoryHandler() http.Handler {
	inv := newInventory()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /items", inv.handleList)
	mux.HandleFunc("POST /items", inv.handleCreate)
	mux.HandleFunc("GET /items/{id}", inv.handleGet)
	mux.HandleFunc("PUT /items/{id}", inv.handleUpdate)
	mux.HandleFunc("DELETE /items/{id}", inv.handleDelete)
	return mux
}

func (inv *inventory) handleCreate(w http.ResponseWriter, r *http.Request) {
	in, ok := readItem(w, r)
	if !ok {
		return
	}
	item, version, err := inv.create(in)
	switch {
	case errors.Is(err, errDuplicate):
		writeProblem(w, http.StatusConflict, "an item with this name already exists")
		return
	case err != nil:
		writeProblem(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Location", "/items/"+item.ID)
	w.Header().Set("ETag", etag(version))
	writeJSON(w, http.StatusCreated, item)
}

func (inv *inventory) handleGet(w http.ResponseWriter, r *http.Request) {
	item, version, err := inv.get(r.PathValue("id"))
	if errors.Is(err, errNotFound) {
		writeProblem(w, http.StatusNotFound, "no item with that id")
		return
	}
	w.Header().Set("ETag", etag(version))
	writeJSON(w, http.StatusOK, item)
}

func (inv *inventory) handleUpdate(w http.ResponseWriter, r *http.Request) {
	ifMatch := r.Header.Get("If-Match")
	if ifMatch == "" {
		writeProblem(w, http.StatusPreconditionRequired, "If-Match header is required")
		return
	}
	in, ok := readItem(w, r)
	if !ok {
		return
	}
	match := func(version int64) bool { return matchETag(ifMatch, version) }
	item, version, err := inv.update(r.PathValue("id"), in, match)
	switch {
	case errors.Is(err, errNotFound):
		writeProblem(w, http.StatusNotFound, "no item with that id")
		return
	case errors.Is(err, errPrecondition):
		writeProblem(w, http.StatusPreconditionFailed, "the item changed; reload and retry")
		return
	case errors.Is(err, errDuplicate):
		writeProblem(w, http.StatusConflict, "an item with this name already exists")
		return
	case err != nil:
		writeProblem(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("ETag", etag(version))
	writeJSON(w, http.StatusOK, item)
}

func (inv *inventory) handleDelete(w http.ResponseWriter, r *http.Request) {
	if err := inv.remove(r.PathValue("id")); errors.Is(err, errNotFound) {
		writeProblem(w, http.StatusNotFound, "no item with that id")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// etag formats a version as a strong entity tag.
func etag(version int64) string {
	return `"` + strconv.FormatInt(version, 10) + `"`
}

// matchETag implements strong comparison of an If-Match value against the
// current version. "*" matches any existing item; weak tags never match.
func matchETag(header string, version int64) bool {
	h := strings.TrimSpace(header)
	if h == "*" {
		return true
	}
	if strings.HasPrefix(h, "W/") {
		return false
	}
	return h == etag(version)
}

// readItem decodes and validates an ItemInput body. On failure it writes the
// problem response itself and reports false.
func readItem(w http.ResponseWriter, r *http.Request) (ItemInput, bool) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeProblem(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return ItemInput{}, false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	dec.DisallowUnknownFields()
	var in ItemInput
	if err := dec.Decode(&in); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeProblem(w, http.StatusRequestEntityTooLarge, "request body too large")
			return ItemInput{}, false
		}
		writeProblem(w, http.StatusBadRequest, "malformed JSON: "+err.Error())
		return ItemInput{}, false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		writeProblem(w, http.StatusBadRequest, "unexpected data after JSON value")
		return ItemInput{}, false
	}

	in.Name = strings.TrimSpace(in.Name)
	errs := FieldErrors{}
	switch n := utf8.RuneCountInString(in.Name); {
	case n == 0:
		errs["name"] = "is required"
	case n > maxNameRunes:
		errs["name"] = "must be at most 64 characters"
	}
	if in.Quantity < 0 {
		errs["quantity"] = "must be >= 0"
	}
	if len(errs) > 0 {
		writeFieldProblem(w, http.StatusUnprocessableEntity, errs)
		return ItemInput{}, false
	}
	return in, true
}

// writeJSON encodes v as the JSON response body.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeProblem writes an RFC 9457 problem with the standard title for status.
func writeProblem(w http.ResponseWriter, status int, detail string) {
	writeProblemDoc(w, Problem{
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Status: status,
		Detail: detail,
	})
}

// writeFieldProblem writes a 422-style problem listing per-field errors.
func writeFieldProblem(w http.ResponseWriter, status int, errs FieldErrors) {
	writeProblemDoc(w, Problem{
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Status: status,
		Detail: "one or more fields are invalid",
		Errors: errs,
	})
}

func writeProblemDoc(w http.ResponseWriter, p Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}
