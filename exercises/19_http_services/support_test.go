package httpsvc_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpsvc "goref/exercises/19_http_services"
)

// do sends one request through h and returns the recorded response.
func do(t *testing.T, h http.Handler, method, target, body string, hdr http.Header) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	for k, vs := range hdr {
		req.Header[k] = vs
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// jsonHeader returns the headers for a JSON request body.
func jsonHeader() http.Header {
	return http.Header{"Content-Type": {"application/json"}}
}

// decodeBody unmarshals the recorded response body into v.
func decodeBody(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
}

// problemOf decodes the problem document in the recorded response.
func problemOf(t *testing.T, rec *httptest.ResponseRecorder) httpsvc.Problem {
	t.Helper()
	var p httpsvc.Problem
	decodeBody(t, rec, &p)
	return p
}

// createItem POSTs a valid item and returns it, failing on any status but 201.
func createItem(t *testing.T, h http.Handler, name string, qty int) httpsvc.Item {
	t.Helper()
	body, err := json.Marshal(httpsvc.ItemInput{Name: name, Quantity: qty})
	if err != nil {
		t.Fatal(err)
	}
	rec := do(t, h, http.MethodPost, "/items", string(body), jsonHeader())
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %q: status %d, body %s", name, rec.Code, rec.Body)
	}
	var item httpsvc.Item
	decodeBody(t, rec, &item)
	return item
}

// blockingWriter is a ResponseWriter whose Write blocks until release is closed.
// It has no Flush method, so http.ResponseController reports it as unsupported.
type blockingWriter struct {
	release chan struct{}
	h       http.Header
}

func (w *blockingWriter) Header() http.Header {
	if w.h == nil {
		w.h = http.Header{}
	}
	return w.h
}

func (w *blockingWriter) WriteHeader(int) {}

func (w *blockingWriter) Write(p []byte) (int, error) {
	<-w.release
	return len(p), nil
}
