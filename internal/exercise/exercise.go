// Package exercise lets tests locate files owned by the engineer.
//
// Go code under test is swapped by `go run ./tools/solutions` through a build
// overlay, but overlays do not affect files read at run time (Dockerfile,
// SQL migrations, testdata written by the engineer, ...). Tests read those via
// [Dir], which points at solutions/<exercise> when GOREF_SOLUTIONS is set.
package exercise

import (
	"os"
	"path/filepath"
	"testing"
)

// Solutions reports whether the run grades the reference solutions.
func Solutions() bool { return os.Getenv("GOREF_SOLUTIONS") != "" }

// Dir returns the directory holding the engineer-owned files of the exercise
// whose tests are running (go test runs tests with the package dir as cwd).
func Dir(t testing.TB) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if !Solutions() {
		return wd
	}
	dir := filepath.Join(wd, "..", "..", "solutions", filepath.Base(wd))
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("no reference solution directory: %v", err)
	}
	return dir
}
