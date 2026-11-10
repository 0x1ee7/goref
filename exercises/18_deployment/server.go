package deployment

import (
	"context"
	"net/http"
)

// ReadyCheck reports whether a dependency is usable: nil means healthy.
// Checks should honour ctx; the server cancels it after 2s.
type ReadyCheck func(ctx context.Context) error

// Deps holds the collaborators injected into [NewServer].
type Deps struct {
	// Readiness checks run on every /readyz request, keyed by name.
	Readiness map[string]ReadyCheck
}

// NewServer returns an unstarted server with these routes:
//
//	GET /healthz  liveness: always 200 {"status":"ok"}; never runs checks
//	GET /readyz   runs every check: 200 when all pass, 503 otherwise
//	GET /version  {"app", "version", "commit"} (commit is null when unknown)
//
// HEAD is accepted wherever GET is. Other methods get 405 with an Allow
// header; unknown paths get 404. Every error body is JSON.
//
// Timeouts: ReadHeaderTimeout 5s, ReadTimeout and WriteTimeout cfg.RequestTimeout,
// IdleTimeout 2m, MaxHeaderBytes 64 KiB, Addr ":PORT".
func NewServer(cfg Config, deps Deps) *http.Server { return nil }
