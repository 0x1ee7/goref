package signatures

import (
	"errors"
	"net/http"
	"time"
)

// DefaultTimeout is the request timeout used when WithTimeout is not given.
const DefaultTimeout = 30 * time.Second

// DefaultUserAgent is sent when WithUserAgent is not given.
const DefaultUserAgent = "goref-client/1"

// ErrInvalidBaseURL is wrapped by NewClient for an unusable base URL.
var ErrInvalidBaseURL = errors.New("invalid base URL")

// ErrInvalidOption is wrapped by an Option that rejects its argument, and by
// NewClient when it is given a nil Option.
var ErrInvalidOption = errors.New("invalid option")

// Config is the resolved client configuration. NewClient fills in defaults
// before applying options, so the zero value of each option-controlled field
// is never observed after construction, except Retries (0 = single attempt)
// and Headers (nil = none).
type Config struct {
	BaseURL   string        // normalized: scheme and host set, no trailing slash
	Timeout   time.Duration // always > 0 after NewClient
	Retries   int           // >= 0
	UserAgent string
	Headers   http.Header // canonical header keys
}

// Option configures a Config. It returns an error (wrapping ErrInvalidOption)
// when its argument is unusable.
type Option func(*Config) error

// WithTimeout sets the request timeout. d must be > 0.
func WithTimeout(d time.Duration) Option {
	return func(*Config) error { return nil }
}

// WithRetries sets how many times a request is retried. n must be >= 0.
func WithRetries(n int) Option {
	return func(*Config) error { return nil }
}

// WithUserAgent sets the User-Agent. ua must be non-empty.
func WithUserAgent(ua string) Option {
	return func(*Config) error { return nil }
}

// WithHeader adds a default header sent with every request. The name is
// canonicalized with http.CanonicalHeaderKey; setting the same name twice
// keeps the last value. Names must be valid HTTP tokens and values must not
// contain CR or LF.
func WithHeader(name, value string) Option {
	return func(*Config) error { return nil }
}

// Client is an HTTP API client. It is configured once by NewClient.
type Client struct{}

// NewClient validates baseURL (http or https, non-empty host, no query or
// fragment, trailing slash removed) and applies opts in order. Defaults are
// set before any option runs. The first failing option aborts construction.
func NewClient(baseURL string, opts ...Option) (*Client, error) {
	return nil, nil
}

// Config returns a copy of the resolved configuration. Mutating the returned
// Headers does not affect the client.
func (c *Client) Config() Config { return Config{} }
