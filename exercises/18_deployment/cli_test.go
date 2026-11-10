package deployment_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	deployment "goref/exercises/18_deployment"
)

// syncBuffer lets a test poll output that Main writes from another goroutine.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// runMain calls deployment.Main synchronously and returns its exit code and output.
func runMain(t *testing.T, args []string, env map[string]string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	code = deployment.Main(ctx, args, getenvFrom(env), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestPart4UsageAndHelp(t *testing.T) {
	t.Run("no arguments is a usage error", func(t *testing.T) {
		code, _, stderr := runMain(t, nil, nil)
		if code != 2 {
			t.Errorf("exit code = %d, want 2", code)
		}
		if !strings.Contains(stderr, "usage") {
			t.Errorf("stderr = %q, want usage text", stderr)
		}
	})

	for _, arg := range []string{"help", "-h"} {
		t.Run(arg+" prints usage to stdout with exit 0", func(t *testing.T) {
			code, stdout, _ := runMain(t, []string{arg}, nil)
			if code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}
			for _, cmd := range []string{"serve", "check-config", "version"} {
				if !strings.Contains(stdout, cmd) {
					t.Errorf("usage on stdout does not mention %q: %q", cmd, stdout)
				}
			}
		})
	}
}

func TestPart4UsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string // substring expected on stderr
	}{
		{"unknown command", []string{"frobnicate"}, "frobnicate"},
		{"version with stray argument", []string{"version", "extra"}, "extra"},
		{"check-config with stray argument", []string{"check-config", "extra"}, "extra"},
		{"serve with unknown flag", []string{"serve", "-nope"}, "nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _, stderr := runMain(t, tt.args, baseEnv())
			if code != 2 {
				t.Errorf("exit code = %d, want 2", code)
			}
			if !strings.Contains(stderr, tt.want) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, tt.want)
			}
		})
	}
}

func TestPart4Version(t *testing.T) {
	origVersion, origCommit := deployment.Version, deployment.Commit
	t.Cleanup(func() { deployment.Version, deployment.Commit = origVersion, origCommit })

	tests := []struct {
		name    string
		version string
		commit  string
		want    string
	}{
		{"version only", "1.2.3", "", "goref-service 1.2.3\n"},
		{"version and commit", "1.2.3", "abc123", "goref-service 1.2.3 (commit abc123)\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployment.Version, deployment.Commit = tt.version, tt.commit
			code, stdout, stderr := runMain(t, []string{"version"}, nil)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr %q", code, stderr)
			}
			if stdout != tt.want {
				t.Errorf("stdout = %q, want %q", stdout, tt.want)
			}
		})
	}
}

func TestPart4CheckConfig(t *testing.T) {
	t.Run("valid environment", func(t *testing.T) {
		code, stdout, stderr := runMain(t, []string{"check-config"}, baseEnv())
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr %q", code, stderr)
		}
		if stdout != "configuration ok\n" {
			t.Errorf("stdout = %q, want %q", stdout, "configuration ok\n")
		}
	})

	t.Run("each problem on its own error line, secrets not echoed", func(t *testing.T) {
		const leak = "tok-leak-probe"
		env := map[string]string{
			"DATABASE_URL": "",
			"PORT":         "x",
			"API_TOKEN":    leak,
		}
		code, stdout, stderr := runMain(t, []string{"check-config"}, env)
		if code != 2 {
			t.Errorf("exit code = %d, want 2", code)
		}
		if stdout != "" {
			t.Errorf("stdout = %q, want nothing on an invalid configuration", stdout)
		}
		var errLines []string
		for _, line := range strings.Split(strings.TrimSpace(stderr), "\n") {
			if strings.HasPrefix(line, "error: ") {
				errLines = append(errLines, line)
			}
		}
		if len(errLines) != 3 {
			t.Fatalf("got %d \"error:\" lines, want 3 (one per bad variable); stderr:\n%s", len(errLines), stderr)
		}
		for _, name := range []string{"DATABASE_URL", "PORT", "API_TOKEN"} {
			if !strings.Contains(stderr, name) {
				t.Errorf("stderr does not name %s:\n%s", name, stderr)
			}
		}
		if strings.Contains(stderr, leak) {
			t.Errorf("stderr echoes the API token:\n%s", stderr)
		}
	})
}

func TestPart4ServeRefusesInvalidConfig(t *testing.T) {
	env := baseEnv()
	delete(env, "DATABASE_URL")
	code, stdout, _ := runMain(t, []string{"serve", "-addr", "127.0.0.1:0"}, env)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if strings.Contains(stdout, "listening") {
		t.Errorf("serve started listening despite invalid configuration: %q", stdout)
	}
}

func TestPart4ServeListenFailureExitsOne(t *testing.T) {
	code, stdout, stderr := runMain(t, []string{"serve", "-addr", "127.0.0.1:notaport"}, baseEnv())
	if code != 1 {
		t.Errorf("exit code = %d, want 1 for a runtime failure; stderr %q", code, stderr)
	}
	if strings.Contains(stdout, "listening on") {
		t.Errorf("serve reported listening although the address is invalid: %q", stdout)
	}
	if strings.TrimSpace(stderr) == "" {
		t.Error("stderr is empty; the listen error should be reported")
	}
}

func TestPart4ServeRunsUntilCancelled(t *testing.T) {
	const token = "0123456789abcdef-secret-token"
	env := baseEnv()
	env["API_TOKEN"] = token

	var stdout, stderr syncBuffer
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	codeCh := make(chan int, 1)
	go func() {
		codeCh <- deployment.Main(ctx, []string{"serve", "-addr", "127.0.0.1:0"}, getenvFrom(env), &stdout, &stderr)
	}()

	// Discover the ephemeral address from the startup line.
	var addr string
	deadline := time.Now().Add(callTimeout)
	for addr == "" && time.Now().Before(deadline) {
		if _, rest, ok := strings.Cut(stdout.String(), "listening on "); ok {
			addr = strings.TrimSpace(strings.SplitN(rest, "\n", 2)[0])
		}
		time.Sleep(10 * time.Millisecond)
	}
	if addr == "" {
		cancel()
		t.Fatalf("serve never printed its listen address; stdout %q, stderr %q", stdout.String(), stderr.String())
	}

	resp, err := (&http.Client{Timeout: time.Second}).Get("http://" + addr + "/healthz")
	if err != nil {
		cancel()
		t.Fatalf("GET /healthz on running server: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/healthz status = %d, want 200", resp.StatusCode)
	}

	cancel()
	select {
	case code := <-codeCh:
		if code != 0 {
			t.Errorf("exit code after SIGINT-style cancel = %d, want 0", code)
		}
	case <-time.After(callTimeout + 5*time.Second):
		t.Fatal("serve did not exit after its context was cancelled")
	}

	if strings.Contains(stderr.String(), token) {
		t.Errorf("startup logs leak the API token:\n%s", stderr.String())
	}
}
