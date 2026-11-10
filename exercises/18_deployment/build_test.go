package deployment_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"goref/internal/exercise"
)

// goToolchain returns the go binary of the toolchain running these tests (GOROOT), not whatever is on PATH.
func goToolchain(t *testing.T) string {
	t.Helper()
	root := runtime.GOROOT()
	if root == "" {
		t.Fatal("runtime.GOROOT() is empty; cannot locate the toolchain")
	}
	bin := filepath.Join(root, "bin", "go")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	return bin
}

// solutionOverlay writes a go build -overlay file that swaps every source file
// of exercises/18_deployment for its counterpart in solutions/18_deployment.
// Subprocess builds do not inherit the harness overlay, so the test provides one.
func solutionOverlay(t *testing.T, root string) string {
	t.Helper()
	exDir := filepath.Join(root, "exercises", "18_deployment")
	solDir := filepath.Join(root, "solutions", "18_deployment")
	replace := map[string]string{}
	err := filepath.WalkDir(solDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(solDir, path)
		if err != nil {
			return err
		}
		replace[filepath.Join(exDir, rel)] = path
		return nil
	})
	if err != nil {
		t.Fatalf("walking reference solution: %v", err)
	}
	data, err := json.Marshal(map[string]any{"Replace": replace})
	if err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(t.TempDir(), "overlay.json")
	if err := os.WriteFile(overlay, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return overlay
}

func TestPart5BuildInjectsVersion(t *testing.T) {
	root := filepath.Join(exercise.Dir(t), "..", "..")
	const (
		pkg    = "./exercises/18_deployment/cmd/goref-service"
		prefix = "goref/exercises/18_deployment"
	)
	ldflags := "-s -w -X " + prefix + ".Version=1.2.3 -X " + prefix + ".Commit=abc123"

	bin := filepath.Join(t.TempDir(), "goref-service")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	args := []string{"build", "-trimpath", "-ldflags", ldflags, "-o", bin}
	if exercise.Solutions() {
		args = append(args, "-overlay", solutionOverlay(t, root))
	}
	args = append(args, pkg)

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, goToolchain(t), args...)
	build.Dir = root
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOTOOLCHAIN=local")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	out, err := exec.CommandContext(ctx, bin, "version").Output()
	if err != nil {
		t.Fatalf("running built binary `version`: %v", err)
	}
	const want = "goref-service 1.2.3 (commit abc123)\n"
	if string(out) != want {
		t.Errorf("binary printed %q, want %q (version and commit injected with -X)", out, want)
	}
}
