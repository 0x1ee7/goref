package httpsvc

import (
	"context"
	"iter"
	"net/http"
	"time"
)

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient replaces the underlying http.Client.
func WithHTTPClient(hc *http.Client) Option {
	return func(*Client) {}
}

// WithToken sends "Authorization: Bearer <token>" on every request.
func WithToken(token string) Option {
	return func(*Client) {}
}

// WithRetries sets how many times a retryable failure is retried.
func WithRetries(n int) Option {
	return func(*Client) {}
}

// WithBackoff sets the base backoff. Attempt i waits base << i unless the
// server sends Retry-After.
func WithBackoff(base time.Duration) Option {
	return func(*Client) {}
}

// Client is a typed client for the inventory API.
type Client struct {
	// engineer-owned fields
}

// NewClient returns a Client for the API rooted at baseURL, which must be an
// absolute http or https URL. Any path in baseURL is kept as a prefix.
func NewClient(baseURL string, opts ...Option) (*Client, error) {
	return nil, errNotImplemented
}

// CreateItem creates an item and returns it.
func (c *Client) CreateItem(ctx context.Context, name string, qty int) (Item, error) {
	return Item{}, errNotImplemented
}

// GetItem returns an item and its ETag.
func (c *Client) GetItem(ctx context.Context, id string) (Item, string, error) {
	return Item{}, "", errNotImplemented
}

// UpdateItem replaces an item if etag still matches. It returns the item and
// the new ETag. An empty etag fails without sending a request.
func (c *Client) UpdateItem(ctx context.Context, id, etag, name string, qty int) (Item, string, error) {
	return Item{}, "", errNotImplemented
}

// DeleteItem deletes an item.
func (c *Client) DeleteItem(ctx context.Context, id string) error {
	return errNotImplemented
}

// ListItems fetches one page of items.
func (c *Client) ListItems(ctx context.Context, q ListQuery) (ItemPage, error) {
	return ItemPage{}, errNotImplemented
}

// AllItems iterates over every matching item, following cursors lazily.
func (c *Client) AllItems(ctx context.Context, q ListQuery) iter.Seq2[Item, error] {
	return func(yield func(Item, error) bool) {
		yield(Item{}, errNotImplemented)
	}
}

// APIError is a non-2xx response decoded from its problem document.
type APIError struct {
	StatusCode int
	Problem    Problem
}

// Error implements error.
func (e *APIError) Error() string {
	return ""
}

// AuthTransport adds a bearer token to each request without mutating the
// caller's request.
type AuthTransport struct {
	Base  http.RoundTripper
	Token string
}

// RoundTrip implements http.RoundTripper.
func (t *AuthTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return nil, errNotImplemented
}

// MetricsTransport reports every round trip to Observe, including failures.
type MetricsTransport struct {
	Base    http.RoundTripper
	Observe func(method string, status int, elapsed time.Duration, err error)
}

// RoundTrip implements http.RoundTripper.
func (t *MetricsTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return nil, errNotImplemented
}

// ParseRetryAfter parses a Retry-After value given as delay seconds or an
// HTTP-date relative to now. It reports false for invalid values.
func ParseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	return 0, false
}
