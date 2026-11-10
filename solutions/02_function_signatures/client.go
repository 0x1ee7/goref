package signatures

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client defaults.
const (
	DefaultTimeout   = 30 * time.Second
	DefaultUserAgent = "goref-client/1"
)

// ErrInvalidBaseURL reports a base URL that is not an absolute http(s) URL with
// a host and no query or fragment.
var ErrInvalidBaseURL = errors.New("invalid base URL")

// ErrInvalidOption reports an option that was nil or rejected its argument.
var ErrInvalidOption = errors.New("invalid client option")

// Config is the resolved client configuration. Callers read it through
// Client.Config, which returns a deep copy.
type Config struct {
	BaseURL   string
	Timeout   time.Duration
	Retries   int
	UserAgent string
	Headers   http.Header
}

// Option configures a Config. Options are applied in order; an option returns
// an error to reject its argument. Validation lives here, not in the
// constructor, so each option owns the rules for its own input.
type Option func(*Config) error

// WithTimeout sets the request timeout. It must be positive.
func WithTimeout(d time.Duration) Option {
	return func(c *Config) error {
		if d <= 0 {
			return fmt.Errorf("%w: timeout must be positive, got %v", ErrInvalidOption, d)
		}
		c.Timeout = d
		return nil
	}
}

// WithRetries sets the number of retries after the first attempt. Zero is valid.
func WithRetries(n int) Option {
	return func(c *Config) error {
		if n < 0 {
			return fmt.Errorf("%w: retries must be >= 0, got %d", ErrInvalidOption, n)
		}
		c.Retries = n
		return nil
	}
}

// WithUserAgent sets the User-Agent header. It must not be empty.
func WithUserAgent(ua string) Option {
	return func(c *Config) error {
		if strings.TrimSpace(ua) == "" {
			return fmt.Errorf("%w: user agent must not be empty", ErrInvalidOption)
		}
		c.UserAgent = ua
		return nil
	}
}

// WithHeader adds a default header. The name is canonicalized, and a later
// WithHeader for the same name replaces the value.
func WithHeader(name, value string) Option {
	return func(c *Config) error {
		if !isToken(name) {
			return fmt.Errorf("%w: header name %q", ErrInvalidOption, name)
		}
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("%w: header %q value contains CR or LF", ErrInvalidOption, name)
		}
		if c.Headers == nil {
			c.Headers = make(http.Header)
		}
		c.Headers.Set(name, value)
		return nil
	}
}

// isToken reports whether s is a non-empty RFC 7230 token.
func isToken(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		ch := s[i]
		switch {
		case 'a' <= ch && ch <= 'z', 'A' <= ch && ch <= 'Z', '0' <= ch && ch <= '9':
		case strings.IndexByte("!#$%&'*+-.^_`|~", ch) >= 0:
		default:
			return false
		}
	}
	return true
}

// Client holds a validated configuration for talking to one API.
type Client struct {
	cfg Config
}

// NewClient validates baseURL, applies defaults, then applies opts in order.
// It returns the first option error and no client.
func NewClient(baseURL string, opts ...Option) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %q: %w", ErrInvalidBaseURL, baseURL, err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return nil, fmt.Errorf("%w: %q", ErrInvalidBaseURL, baseURL)
	}

	cfg := Config{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		Timeout:   DefaultTimeout,
		UserAgent: DefaultUserAgent,
	}
	for i, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("%w: option %d is nil", ErrInvalidOption, i)
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}
	return &Client{cfg: cfg}, nil
}

// Config returns a copy of the resolved configuration. Mutating the returned
// value, including its Headers, does not affect the client.
func (c *Client) Config() Config {
	cfg := c.cfg
	cfg.Headers = c.cfg.Headers.Clone()
	return cfg
}
