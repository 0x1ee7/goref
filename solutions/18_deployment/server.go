package deployment

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"time"
)

// Server hardening constants. RequestTimeout (from Config) bounds reads and writes.
const (
	readHeaderTimeout     = 5 * time.Second
	idleTimeout           = 2 * time.Minute
	maxHeaderBytes        = 1 << 16
	readinessCheckTimeout = 2 * time.Second
)

// ReadyCheck reports whether a dependency is usable: nil means healthy.
// Checks should honour ctx; the server cancels it after readinessCheckTimeout.
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
func NewServer(cfg Config, deps Deps) *http.Server {
	a := api{cfg: cfg, checks: deps.Readiness}

	mux := http.NewServeMux()
	// Method-less patterns: ServeMux's own 405 is plain text, so the method
	// check lives in the handlers where the response format is ours.
	mux.HandleFunc("/healthz", a.healthz)
	mux.HandleFunc("/readyz", a.readyz)
	mux.HandleFunc("/version", a.version)
	mux.HandleFunc("/", notFound)

	return &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       cfg.RequestTimeout,
		WriteTimeout:      cfg.RequestTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}
}

type api struct {
	cfg    Config
	checks map[string]ReadyCheck
}

func (a api) healthz(w http.ResponseWriter, r *http.Request) {
	if !allowRead(w, r) {
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
}

func (a api) readyz(w http.ResponseWriter, r *http.Request) {
	if !allowRead(w, r) {
		return
	}
	results := make(map[string]string, len(a.checks))
	ready := true
	// Sorted order keeps the response deterministic; every check runs even after a failure.
	for _, name := range slices.Sorted(maps.Keys(a.checks)) {
		if err := runCheck(r.Context(), a.checks[name]); err != nil {
			ready = false
			results[name] = "error: " + err.Error()
			continue
		}
		results[name] = "ok"
	}
	if !ready {
		writeJSON(w, r, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "checks": results})
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"status": "ready", "checks": results})
}

// runCheck bounds one check. A check that ignores its context is not interrupted.
func runCheck(ctx context.Context, check ReadyCheck) error {
	ctx, cancel := context.WithTimeout(ctx, readinessCheckTimeout)
	defer cancel()
	return check(ctx)
}

func (a api) version(w http.ResponseWriter, r *http.Request) {
	if !allowRead(w, r) {
		return
	}
	var commit any // stays nil -> JSON null
	if c := BuildCommit(); c != "" {
		commit = c
	}
	writeJSON(w, r, http.StatusOK, map[string]any{
		"app":     a.cfg.AppName,
		"version": BuildVersion(),
		"commit":  commit,
	})
}

func notFound(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusNotFound, map[string]string{"error": "not_found", "path": r.URL.Path})
}

// allowRead answers non-GET/HEAD requests with 405 and reports whether the handler should continue.
func allowRead(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return true
	}
	w.Header().Set("Allow", "GET, HEAD")
	writeJSON(w, r, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
	return false
}

// writeJSON marshals before writing so Content-Length is exact. HEAD gets the same headers and no body.
func writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}
