// Command solutions runs the exercise tests against the reference
// implementations in solutions/ without touching your code.
//
// It builds a `go build -overlay` file that, for every exercise with a
// reference solution, swaps your non-test .go files for the files in
// solutions/<exercise>/, then runs `go test -overlay=<file> <args>` with
// GOREF_SOLUTIONS=1 so tests that read non-Go files (Dockerfile, SQL, ...)
// can switch directories via goref/internal/exercise.
//
//	go run ./tools/solutions                                   # every exercise
//	go run ./tools/solutions ./exercises/05_iterators/ -run Part2 -v
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	code, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "solutions:", err)
		os.Exit(2)
	}
	os.Exit(code)
}

func run(args []string) (int, error) {
	root, err := os.Getwd()
	if err != nil {
		return 0, err
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return 0, errors.New("run from the repository root (go.mod not found)")
	}

	replace, err := overlay(root)
	if err != nil {
		return 0, err
	}
	data, err := json.MarshalIndent(map[string]any{"Replace": replace}, "", "  ")
	if err != nil {
		return 0, err
	}
	f, err := os.CreateTemp("", "goref-overlay-*.json")
	if err != nil {
		return 0, err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return 0, err
	}
	if err := f.Close(); err != nil {
		return 0, err
	}

	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		args = append([]string{"./exercises/..."}, args...)
	}
	cmd := exec.Command(goBinary(), append([]string{"test", "-overlay=" + f.Name()}, args...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), "GOREF_SOLUTIONS=1")
	if err := cmd.Run(); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return exitErr.ExitCode(), nil
		}
		return 0, err
	}
	return 0, nil
}

// overlay maps exercises/<ex>/<file>.go to solutions/<ex>/<file>.go and hides
// (maps to "") every non-test .go file of <ex> that the solution does not have.
func overlay(root string) (map[string]string, error) {
	solutions := filepath.Join(root, "solutions")
	exercises := filepath.Join(root, "exercises")
	replace := map[string]string{}

	entries, err := os.ReadDir(solutions)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		solDir := filepath.Join(solutions, e.Name())
		exDir := filepath.Join(exercises, e.Name())
		err := filepath.WalkDir(exDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !isSource(path) {
				return err
			}
			replace[path] = "" // hidden unless the solution provides it
			return nil
		})
		if err != nil {
			return nil, err
		}
		err = filepath.WalkDir(solDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !isSource(path) {
				return err
			}
			rel, err := filepath.Rel(solDir, path)
			if err != nil {
				return err
			}
			replace[filepath.Join(exDir, rel)] = path
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return replace, nil
}

func isSource(path string) bool {
	return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
}

// goBinary prefers the toolchain that is running this command over whatever
// `go` happens to be first on PATH.
func goBinary() string {
	if goroot := os.Getenv("GOROOT"); goroot != "" {
		candidate := filepath.Join(goroot, "bin", "go")
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate
		}
	}
	return "go"
}
