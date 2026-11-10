// Package httpsvc is the 19_http_services exercise: an inventory REST API,
// its middleware, cursor pagination, a retrying HTTP client and an NDJSON
// event stream. See README.md for the full spec.
package httpsvc

import (
	"errors"
	"net/http"
)

// errNotImplemented is returned by stubs until the engineer implements them.
var errNotImplemented = errors.New("not implemented")

// MaxBodyBytes caps the size of request bodies accepted by the API.
const MaxBodyBytes = 1 << 20

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
func (f FieldErrors) Error() string { return "" }

// NewInventoryHandler returns the REST API for items, mounted at /items.
func NewInventoryHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
	})
}
