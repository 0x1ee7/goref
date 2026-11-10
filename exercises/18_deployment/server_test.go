package deployment_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	deployment "goref/exercises/18_deployment"
)

// newTestServer builds a server for cfg from baseEnv and fails the test if NewServer returns nil.
func newTestServer(t *testing.T, cfg deployment.Config, deps deployment.Deps) *http.Server {
	t.Helper()
	srv := deployment.NewServer(cfg, deps)
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
	return srv
}

func loadTestConfig(t *testing.T) deployment.Config {
	t.Helper()
	cfg, err := deployment.LoadConfig(getenvFrom(baseEnv()))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return cfg
}

// do sends one request through the server's handler, without a socket.
func do(t *testing.T, srv *http.Server, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q is not a JSON object: %v", rec.Body.String(), err)
	}
	return body
}

func requireJSONContentType(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json; charset=utf-8", ct)
	}
}

func TestPart2HealthzLiveness(t *testing.T) {
	var checks atomic.Int32
	srv := newTestServer(t, loadTestConfig(t), deployment.Deps{
		Readiness: map[string]deployment.ReadyCheck{
			"db": func(context.Context) error { checks.Add(1); return errors.New("down") },
		},
	})

	t.Run("GET is 200 with status ok", func(t *testing.T) {
		rec := do(t, srv, http.MethodGet, "/healthz")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		requireJSONContentType(t, rec)
		if diff := cmp.Diff(map[string]any{"status": "ok"}, decodeJSON(t, rec)); diff != "" {
			t.Errorf("body (-want +got):\n%s", diff)
		}
	})

	t.Run("HEAD is 200 with headers and no body", func(t *testing.T) {
		rec := do(t, srv, http.MethodHead, "/healthz")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("HEAD body = %q, want empty", rec.Body.String())
		}
	})

	t.Run("POST is 405 with Allow", func(t *testing.T) {
		rec := do(t, srv, http.MethodPost, "/healthz")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want 405", rec.Code)
		}
		if allow := rec.Header().Get("Allow"); allow != "GET, HEAD" {
			t.Errorf("Allow = %q, want %q", allow, "GET, HEAD")
		}
		requireJSONContentType(t, rec)
	})

	t.Run("never runs readiness checks", func(t *testing.T) {
		if n := checks.Load(); n != 0 {
			t.Errorf("readiness checks ran %d times during /healthz; liveness must not depend on dependencies", n)
		}
	})
}

func TestPart2ReadyzReportsChecks(t *testing.T) {
	t.Run("all healthy is 200 ready", func(t *testing.T) {
		srv := newTestServer(t, loadTestConfig(t), deployment.Deps{
			Readiness: map[string]deployment.ReadyCheck{
				"db":    func(context.Context) error { return nil },
				"cache": func(context.Context) error { return nil },
			},
		})
		rec := do(t, srv, http.MethodGet, "/readyz")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
		}
		requireJSONContentType(t, rec)
		want := map[string]any{
			"status": "ready",
			"checks": map[string]any{"db": "ok", "cache": "ok"},
		}
		if diff := cmp.Diff(want, decodeJSON(t, rec)); diff != "" {
			t.Errorf("body (-want +got):\n%s", diff)
		}
	})

	t.Run("one failing check is 503 with details", func(t *testing.T) {
		srv := newTestServer(t, loadTestConfig(t), deployment.Deps{
			Readiness: map[string]deployment.ReadyCheck{
				"db":    func(context.Context) error { return errors.New("connection refused") },
				"cache": func(context.Context) error { return nil },
			},
		})
		rec := do(t, srv, http.MethodGet, "/readyz")
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rec.Code)
		}
		requireJSONContentType(t, rec)
		body := decodeJSON(t, rec)
		if body["status"] != "not_ready" {
			t.Errorf("status field = %v, want not_ready", body["status"])
		}
		checks, _ := body["checks"].(map[string]any)
		if checks["cache"] != "ok" {
			t.Errorf("checks[cache] = %v, want ok", checks["cache"])
		}
		if got, _ := checks["db"].(string); got != "error: connection refused" {
			t.Errorf("checks[db] = %v, want %q", checks["db"], "error: connection refused")
		}
	})

	t.Run("every check runs even after a failure", func(t *testing.T) {
		var ran atomic.Int32
		ok := func(context.Context) error { ran.Add(1); return nil }
		bad := func(context.Context) error { ran.Add(1); return errors.New("boom") }
		srv := newTestServer(t, loadTestConfig(t), deployment.Deps{
			Readiness: map[string]deployment.ReadyCheck{"a": bad, "b": ok, "c": bad},
		})
		do(t, srv, http.MethodGet, "/readyz")
		if n := ran.Load(); n != 3 {
			t.Errorf("checks ran %d times, want 3", n)
		}
	})

	t.Run("no checks is ready", func(t *testing.T) {
		srv := newTestServer(t, loadTestConfig(t), deployment.Deps{})
		rec := do(t, srv, http.MethodGet, "/readyz")
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("checks receive a deadline and a hung check cannot stall readyz", func(t *testing.T) {
		var hadDeadline atomic.Bool
		srv := newTestServer(t, loadTestConfig(t), deployment.Deps{
			Readiness: map[string]deployment.ReadyCheck{
				"hung": func(ctx context.Context) error {
					_, ok := ctx.Deadline()
					hadDeadline.Store(ok)
					<-ctx.Done()
					return ctx.Err()
				},
			},
		})
		start := time.Now()
		rec := do(t, srv, http.MethodGet, "/readyz")
		elapsed := time.Since(start)
		if !hadDeadline.Load() {
			t.Error("check context has no deadline")
		}
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503 for a hung check", rec.Code)
		}
		if elapsed > 5*time.Second {
			t.Errorf("readyz took %v with a hung check; the per-check timeout should be about 2s", elapsed)
		}
	})

	t.Run("HEAD is accepted, POST is 405", func(t *testing.T) {
		srv := newTestServer(t, loadTestConfig(t), deployment.Deps{})
		if rec := do(t, srv, http.MethodHead, "/readyz"); rec.Code != http.StatusOK {
			t.Errorf("HEAD status = %d, want 200", rec.Code)
		}
		if rec := do(t, srv, http.MethodPost, "/readyz"); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST status = %d, want 405", rec.Code)
		}
	})
}

func TestPart2VersionEndpoint(t *testing.T) {
	cfg := loadTestConfig(t)
	srv := newTestServer(t, cfg, deployment.Deps{})

	origVersion, origCommit := deployment.Version, deployment.Commit
	t.Cleanup(func() { deployment.Version, deployment.Commit = origVersion, origCommit })

	t.Run("injected version and commit are reported", func(t *testing.T) {
		deployment.Version, deployment.Commit = "1.2.3", "abc123"
		rec := do(t, srv, http.MethodGet, "/version")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		requireJSONContentType(t, rec)
		want := map[string]any{"app": "goref-service", "version": "1.2.3", "commit": "abc123"}
		if diff := cmp.Diff(want, decodeJSON(t, rec)); diff != "" {
			t.Errorf("body (-want +got):\n%s", diff)
		}
	})

	t.Run("unknown commit is JSON null", func(t *testing.T) {
		deployment.Version, deployment.Commit = "1.2.3", ""
		body := decodeJSON(t, do(t, srv, http.MethodGet, "/version"))
		if fallback := deployment.BuildCommit(); fallback == "" {
			if v, ok := body["commit"]; !ok || v != nil {
				t.Errorf("commit = %#v, want null when no commit is known", v)
			}
		}
	})

	t.Run("version falls back to build info, never empty or (devel)", func(t *testing.T) {
		deployment.Version, deployment.Commit = "", ""
		v := deployment.BuildVersion()
		if v == "" || v == "(devel)" {
			t.Errorf("BuildVersion() = %q, want a usable fallback such as dev or the module version", v)
		}
	})

	t.Run("app name comes from config", func(t *testing.T) {
		env := baseEnv()
		env["APP_NAME"] = "billing"
		c, err := deployment.LoadConfig(getenvFrom(env))
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		deployment.Version = "1.0.0"
		body := decodeJSON(t, do(t, newTestServer(t, c, deployment.Deps{}), http.MethodGet, "/version"))
		if body["app"] != "billing" {
			t.Errorf("app = %v, want billing", body["app"])
		}
	})
}

func TestPart2NotFoundAndMethodErrors(t *testing.T) {
	srv := newTestServer(t, loadTestConfig(t), deployment.Deps{})

	t.Run("unknown path is JSON 404", func(t *testing.T) {
		rec := do(t, srv, http.MethodGet, "/no/such/path")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
		requireJSONContentType(t, rec)
		body := decodeJSON(t, rec)
		if body["error"] != "not_found" {
			t.Errorf("error field = %v, want not_found", body["error"])
		}
		if body["path"] != "/no/such/path" {
			t.Errorf("path field = %v, want /no/such/path", body["path"])
		}
	})

	t.Run("wrong method on version is JSON 405 with Allow", func(t *testing.T) {
		rec := do(t, srv, http.MethodDelete, "/version")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want 405", rec.Code)
		}
		requireJSONContentType(t, rec)
		if allow := rec.Header().Get("Allow"); allow != "GET, HEAD" {
			t.Errorf("Allow = %q, want %q", allow, "GET, HEAD")
		}
		if body := decodeJSON(t, rec); body["error"] != "method_not_allowed" {
			t.Errorf("error field = %v, want method_not_allowed", body["error"])
		}
	})

	t.Run("root path is not a silent 200", func(t *testing.T) {
		if rec := do(t, srv, http.MethodGet, "/"); rec.Code != http.StatusNotFound {
			t.Errorf("GET / status = %d, want 404", rec.Code)
		}
	})
}

func TestPart2ServerHardening(t *testing.T) {
	env := baseEnv()
	env["REQUEST_TIMEOUT"] = "12s"
	env["PORT"] = "9000"
	cfg, err := deployment.LoadConfig(getenvFrom(env))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	srv := newTestServer(t, cfg, deployment.Deps{})

	if srv.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("ReadHeaderTimeout = %v, want 5s (slowloris guard)", srv.ReadHeaderTimeout)
	}
	if srv.ReadTimeout != 12*time.Second {
		t.Errorf("ReadTimeout = %v, want the REQUEST_TIMEOUT value 12s", srv.ReadTimeout)
	}
	if srv.WriteTimeout != 12*time.Second {
		t.Errorf("WriteTimeout = %v, want the REQUEST_TIMEOUT value 12s", srv.WriteTimeout)
	}
	if srv.IdleTimeout != 2*time.Minute {
		t.Errorf("IdleTimeout = %v, want 2m", srv.IdleTimeout)
	}
	if srv.MaxHeaderBytes != 1<<16 {
		t.Errorf("MaxHeaderBytes = %d, want 65536", srv.MaxHeaderBytes)
	}
	if srv.Addr != ":9000" {
		t.Errorf("Addr = %q, want %q", srv.Addr, ":9000")
	}
}
