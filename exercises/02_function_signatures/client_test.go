package signatures_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"goref/exercises/02_function_signatures"
)

func mustClient(t *testing.T, baseURL string, opts ...signatures.Option) *signatures.Client {
	t.Helper()
	c, err := signatures.NewClient(baseURL, opts...)
	if err != nil {
		t.Fatalf("NewClient(%q): unexpected error: %v", baseURL, err)
	}
	if c == nil {
		t.Fatalf("NewClient(%q) returned nil client without error", baseURL)
	}
	return c
}

// defaults is the configuration NewClient must produce when no option is given.
func defaults(baseURL string) signatures.Config {
	return signatures.Config{
		BaseURL:   baseURL,
		Timeout:   signatures.DefaultTimeout,
		Retries:   0,
		UserAgent: signatures.DefaultUserAgent,
	}
}

func TestPart2Defaults(t *testing.T) {
	const url = "https://api.example.com"
	got := mustClient(t, url).Config()
	if diff := cmp.Diff(defaults(url), got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("Config() mismatch (-want +got):\n%s", diff)
	}
	if signatures.DefaultTimeout != 30*time.Second {
		t.Errorf("DefaultTimeout = %v, want 30s", signatures.DefaultTimeout)
	}
}

func TestPart2ResolvedConfig(t *testing.T) {
	const url = "https://api.example.com"
	tests := []struct {
		name    string
		baseURL string
		opts    []signatures.Option
		want    signatures.Config
	}{
		{
			name:    "timeout",
			baseURL: url,
			opts:    []signatures.Option{signatures.WithTimeout(2 * time.Second)},
			want:    func() signatures.Config { c := defaults(url); c.Timeout = 2 * time.Second; return c }(),
		},
		{
			name:    "zero retries is a valid single attempt",
			baseURL: url,
			opts:    []signatures.Option{signatures.WithRetries(0)},
			want:    defaults(url),
		},
		{
			name:    "retries",
			baseURL: url,
			opts:    []signatures.Option{signatures.WithRetries(3)},
			want:    func() signatures.Config { c := defaults(url); c.Retries = 3; return c }(),
		},
		{
			name:    "user agent",
			baseURL: url,
			opts:    []signatures.Option{signatures.WithUserAgent("probe/2")},
			want:    func() signatures.Config { c := defaults(url); c.UserAgent = "probe/2"; return c }(),
		},
		{
			name:    "options apply in order so the last user agent wins",
			baseURL: url,
			opts:    []signatures.Option{signatures.WithUserAgent("a"), signatures.WithUserAgent("b")},
			want:    func() signatures.Config { c := defaults(url); c.UserAgent = "b"; return c }(),
		},
		{
			name:    "header name is canonicalized",
			baseURL: url,
			opts:    []signatures.Option{signatures.WithHeader("x-request-source", "cli")},
			want: func() signatures.Config {
				c := defaults(url)
				c.Headers = http.Header{"X-Request-Source": {"cli"}}
				return c
			}(),
		},
		{
			name:    "same header name twice keeps the last value",
			baseURL: url,
			opts:    []signatures.Option{signatures.WithHeader("X-Env", "a"), signatures.WithHeader("x-env", "b")},
			want: func() signatures.Config {
				c := defaults(url)
				c.Headers = http.Header{"X-Env": {"b"}}
				return c
			}(),
		},
		{
			name:    "distinct headers accumulate",
			baseURL: url,
			opts:    []signatures.Option{signatures.WithHeader("A-One", "1"), signatures.WithHeader("B-Two", "2")},
			want: func() signatures.Config {
				c := defaults(url)
				c.Headers = http.Header{"A-One": {"1"}, "B-Two": {"2"}}
				return c
			}(),
		},
		{
			name:    "trailing slash is removed from base URL",
			baseURL: "https://api.example.com/v1/",
			want:    defaults("https://api.example.com/v1"),
		},
		{
			name:    "http scheme is accepted",
			baseURL: "http://localhost:8080",
			want:    defaults("http://localhost:8080"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mustClient(t, tc.baseURL, tc.opts...).Config()
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("Config() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPart2InvalidInput(t *testing.T) {
	const url = "https://api.example.com"
	tests := []struct {
		name    string
		baseURL string
		opts    []signatures.Option
		wantErr error
	}{
		{name: "empty base URL", baseURL: "", wantErr: signatures.ErrInvalidBaseURL},
		{name: "no scheme", baseURL: "api.example.com", wantErr: signatures.ErrInvalidBaseURL},
		{name: "unsupported scheme", baseURL: "ftp://files.example.com", wantErr: signatures.ErrInvalidBaseURL},
		{name: "missing host", baseURL: "https://", wantErr: signatures.ErrInvalidBaseURL},
		{name: "query rejected", baseURL: "https://api.example.com?x=1", wantErr: signatures.ErrInvalidBaseURL},
		{name: "fragment rejected", baseURL: "https://api.example.com/#frag", wantErr: signatures.ErrInvalidBaseURL},
		{name: "zero timeout", baseURL: url, opts: []signatures.Option{signatures.WithTimeout(0)}, wantErr: signatures.ErrInvalidOption},
		{name: "negative timeout", baseURL: url, opts: []signatures.Option{signatures.WithTimeout(-time.Second)}, wantErr: signatures.ErrInvalidOption},
		{name: "negative retries", baseURL: url, opts: []signatures.Option{signatures.WithRetries(-1)}, wantErr: signatures.ErrInvalidOption},
		{name: "empty user agent", baseURL: url, opts: []signatures.Option{signatures.WithUserAgent("")}, wantErr: signatures.ErrInvalidOption},
		{name: "header name with space", baseURL: url, opts: []signatures.Option{signatures.WithHeader("bad name", "x")}, wantErr: signatures.ErrInvalidOption},
		{name: "empty header name", baseURL: url, opts: []signatures.Option{signatures.WithHeader("", "x")}, wantErr: signatures.ErrInvalidOption},
		{name: "header value with CRLF", baseURL: url, opts: []signatures.Option{signatures.WithHeader("X-Ok", "a\r\nInjected: b")}, wantErr: signatures.ErrInvalidOption},
		{name: "nil option", baseURL: url, opts: []signatures.Option{nil}, wantErr: signatures.ErrInvalidOption},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := signatures.NewClient(tc.baseURL, tc.opts...)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("NewClient(%q) error = %v, want errors.Is %v", tc.baseURL, err, tc.wantErr)
			}
			if c != nil {
				t.Errorf("NewClient returned a non-nil client alongside an error")
			}
		})
	}
}

func TestPart2FirstFailingOptionAborts(t *testing.T) {
	c, err := signatures.NewClient("https://api.example.com",
		signatures.WithTimeout(0),
		signatures.WithRetries(2),
	)
	if !errors.Is(err, signatures.ErrInvalidOption) {
		t.Fatalf("NewClient error = %v, want errors.Is ErrInvalidOption", err)
	}
	if c != nil {
		t.Errorf("NewClient returned a client after a failing option")
	}
}

func TestPart2ConfigIsACopy(t *testing.T) {
	c := mustClient(t, "https://api.example.com", signatures.WithHeader("X-Keep", "1"))

	cfg := c.Config()
	cfg.Headers.Set("X-Evil", "1")
	cfg.Headers["X-Keep"][0] = "changed"

	want := http.Header{"X-Keep": {"1"}}
	if diff := cmp.Diff(want, c.Config().Headers, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("client config changed through a returned copy (-want +got):\n%s", diff)
	}
}
