package httpsvc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient replaces the underlying http.Client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.hc = hc }
}

// WithToken sends "Authorization: Bearer <token>" on every request.
func WithToken(token string) Option {
	return func(c *Client) { c.token = token }
}

// WithRetries sets how many times a retryable failure is retried. Negative
// values mean no retries.
func WithRetries(n int) Option {
	return func(c *Client) { c.retries = max(n, 0) }
}

// WithBackoff sets the base backoff. Attempt i waits base << i unless the
// server sends Retry-After.
func WithBackoff(base time.Duration) Option {
	return func(c *Client) { c.backoff = base }
}

// Client is a typed client for the inventory API.
type Client struct {
	base    *url.URL
	hc      *http.Client
	token   string
	retries int
	backoff time.Duration
}

// NewClient returns a Client for the API rooted at baseURL, which must be an
// absolute http or https URL. Any path in baseURL is kept as a prefix.
func NewClient(baseURL string, opts ...Option) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("base url %q: must be an absolute http or https URL", baseURL)
	}
	c := &Client{
		base:    u,
		hc:      &http.Client{Timeout: 10 * time.Second},
		retries: 3,
		backoff: 100 * time.Millisecond,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// endpoint joins elems onto the base path and attaches query parameters.
func (c *Client) endpoint(q url.Values, elems ...string) string {
	u := c.base.JoinPath(elems...)
	u.RawQuery = q.Encode()
	return u.String()
}

// CreateItem creates an item and returns it.
func (c *Client) CreateItem(ctx context.Context, name string, qty int) (Item, error) {
	body, err := json.Marshal(ItemInput{Name: name, Quantity: qty})
	if err != nil {
		return Item{}, fmt.Errorf("encode item: %w", err)
	}
	resp, err := c.do(ctx, http.MethodPost, c.endpoint(nil, "items"), body, jsonHeader())
	if err != nil {
		return Item{}, err
	}
	var item Item
	if err := decodeResponse(resp, &item); err != nil {
		return Item{}, err
	}
	return item, nil
}

// GetItem returns an item and its ETag.
func (c *Client) GetItem(ctx context.Context, id string) (Item, string, error) {
	resp, err := c.do(ctx, http.MethodGet, c.endpoint(nil, "items", id), nil, nil)
	if err != nil {
		return Item{}, "", err
	}
	tag := resp.Header.Get("ETag")
	var item Item
	if err := decodeResponse(resp, &item); err != nil {
		return Item{}, "", err
	}
	return item, tag, nil
}

// UpdateItem replaces an item if etag still matches. It returns the item and
// the new ETag. An empty etag fails without sending a request.
func (c *Client) UpdateItem(ctx context.Context, id, etag, name string, qty int) (Item, string, error) {
	if etag == "" {
		return Item{}, "", errors.New("update item: etag is required")
	}
	body, err := json.Marshal(ItemInput{Name: name, Quantity: qty})
	if err != nil {
		return Item{}, "", fmt.Errorf("encode item: %w", err)
	}
	hdr := jsonHeader()
	hdr.Set("If-Match", etag)
	resp, err := c.do(ctx, http.MethodPut, c.endpoint(nil, "items", id), body, hdr)
	if err != nil {
		return Item{}, "", err
	}
	tag := resp.Header.Get("ETag")
	var item Item
	if err := decodeResponse(resp, &item); err != nil {
		return Item{}, "", err
	}
	return item, tag, nil
}

// DeleteItem deletes an item.
func (c *Client) DeleteItem(ctx context.Context, id string) error {
	resp, err := c.do(ctx, http.MethodDelete, c.endpoint(nil, "items", id), nil, nil)
	if err != nil {
		return err
	}
	return decodeResponse(resp, nil)
}

// ListItems fetches one page of items.
func (c *Client) ListItems(ctx context.Context, q ListQuery) (ItemPage, error) {
	resp, err := c.do(ctx, http.MethodGet, c.endpoint(q.Values(), "items"), nil, nil)
	if err != nil {
		return ItemPage{}, err
	}
	var page ItemPage
	if err := decodeResponse(resp, &page); err != nil {
		return ItemPage{}, err
	}
	return page, nil
}

// AllItems iterates over every matching item, following cursors lazily.
func (c *Client) AllItems(ctx context.Context, q ListQuery) iter.Seq2[Item, error] {
	return func(yield func(Item, error) bool) {
		cur := q
		for {
			page, err := c.ListItems(ctx, cur)
			if err != nil {
				yield(Item{}, err)
				return
			}
			for _, item := range page.Items {
				if !yield(item, nil) {
					return
				}
			}
			if page.NextCursor == "" {
				return
			}
			cur.Cursor = page.NextCursor
		}
	}
}

// do sends a request with retries. body is re-sent on every attempt.
func (c *Client) do(ctx context.Context, method, target string, body []byte, hdr http.Header) (*http.Response, error) {
	idempotent := isIdempotent(method)
	for attempt := 0; ; attempt++ {
		var rd io.Reader
		if body != nil {
			rd = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, target, rd)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		for k, vs := range hdr {
			req.Header[k] = slices.Clone(vs)
		}
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}

		resp, err := c.hc.Do(req)
		var wait time.Duration
		if err != nil {
			if ctx.Err() != nil || !idempotent || attempt >= c.retries {
				return nil, fmt.Errorf("%s %s: %w", method, target, err)
			}
			wait = c.backoffFor(attempt)
		} else {
			retryable := resp.StatusCode == http.StatusTooManyRequests ||
				(idempotent && resp.StatusCode >= 500)
			if !retryable || attempt >= c.retries {
				return resp, nil
			}
			wait = c.backoffFor(attempt)
			if d, ok := ParseRetryAfter(resp.Header.Get("Retry-After"), time.Now()); ok {
				wait = d
			}
			closeBody(resp.Body)
		}
		if err := sleep(ctx, wait); err != nil {
			return nil, err
		}
	}
}

func (c *Client) backoffFor(attempt int) time.Duration {
	return c.backoff * time.Duration(1<<attempt)
}

// isIdempotent reports whether retrying method cannot repeat a side effect.
func isIdempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete:
		return true
	}
	return false
}

func jsonHeader() http.Header {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("Accept", "application/json")
	return h
}


// sleep waits for d or until ctx is done, whichever comes first.
func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// closeBody drains a bounded amount of the body so the connection can be
// reused, then closes it.
func closeBody(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 64<<10))
	_ = body.Close()
}

// decodeResponse closes resp. A non-2xx status becomes an *APIError; otherwise
// the body is decoded into out when out is non-nil.
func decodeResponse(resp *http.Response, out any) error {
	defer closeBody(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return readAPIError(resp)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// readAPIError builds an *APIError from a non-2xx response. The body is not
// closed here.
func readAPIError(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	apiErr := &APIError{StatusCode: resp.StatusCode}
	if err := json.Unmarshal(raw, &apiErr.Problem); err != nil || apiErr.Problem.Title == "" {
		apiErr.Problem = Problem{
			Type:   "about:blank",
			Title:  http.StatusText(resp.StatusCode),
			Detail: strings.TrimSpace(string(raw)),
		}
	}
	if apiErr.Problem.Status == 0 {
		apiErr.Problem.Status = resp.StatusCode
	}
	return apiErr
}

// APIError is a non-2xx response decoded from its problem document.
type APIError struct {
	StatusCode int
	Problem    Problem
}

// Error implements error.
func (e *APIError) Error() string {
	detail := e.Problem.Detail
	if detail == "" {
		detail = e.Problem.Title
	}
	return fmt.Sprintf("api error %d: %s", e.StatusCode, detail)
}

// AuthTransport adds a bearer token to each request without mutating the
// caller's request.
type AuthTransport struct {
	Base  http.RoundTripper
	Token string
}

// RoundTrip implements http.RoundTripper.
func (t *AuthTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	req := r.Clone(r.Context())
	req.Header.Set("Authorization", "Bearer "+t.Token)
	return baseTransport(t.Base).RoundTrip(req)
}

// MetricsTransport reports every round trip to Observe, including failures.
type MetricsTransport struct {
	Base    http.RoundTripper
	Observe func(method string, status int, elapsed time.Duration, err error)
}

// RoundTrip implements http.RoundTripper.
func (t *MetricsTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := baseTransport(t.Base).RoundTrip(r)
	if t.Observe != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Observe(r.Method, status, time.Since(start), err)
	}
	return resp, err
}

func baseTransport(rt http.RoundTripper) http.RoundTripper {
	if rt == nil {
		return http.DefaultTransport
	}
	return rt
}

// ParseRetryAfter parses a Retry-After value given as delay seconds or an
// HTTP-date relative to now. It reports false for invalid values.
func ParseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(value); err == nil {
		if secs < 0 {
			return 0, false
		}
		return time.Duration(secs) * time.Second, true
	}
	t, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	return max(t.Sub(now), 0), true
}
