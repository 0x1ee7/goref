// The grader runs your test suite against the billing package and against
// deliberately broken copies of it. It is not part of the exercise to edit.
package testing16

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"goref/internal/exercise"
)

// billingImport is how the suite imports billing in the repository. The grader
// rewrites it to the import path of its temporary module.
const billingImport = "goref/exercises/16_testing/billing"

// minTests is the number of passing tests and subtests a suite must have.
const minTests = 40

// mutant is one deliberate bug: old is replaced by new exactly once in billing.go.
type mutant struct {
	name string
	old  string
	new  string
}

var mutants = []mutant{
	{"round half down", "return (2*num + den) / (2 * den)", "return (2*num + den - 1) / (2 * den)"},
	{"tier threshold exclusive", "if subtotal >= tier.minSubtotal {", "if subtotal > tier.minSubtotal {"},
	{"two-percent tier at 2.5%", "{10_000, 200},", "{10_000, 250},"},
	{"tax on gross subtotal", "tax := roundDiv(taxable*inv.taxBPS, basisPoints)", "tax := roundDiv(subtotal*inv.taxBPS, basisPoints)"},
	{"zero quantity accepted", "case quantity < 1:", "case quantity < 0:"},
	{"100 percent tax accepted", "spec.TaxBPS >= basisPoints", "spec.TaxBPS > basisPoints"},
	{"free invoice charged", "if totals.Total <= 0 {", "if totals.Total < 0 {"},
	{"UTC conversion skipped", "u := t.UTC()", "u := t"},
	{"overdue on due day", "return utcDate(clock.Now()).After(utcDate(due))", "return !utcDate(clock.Now()).Before(utcDate(due))"},
	{"backoff not capped", "return min(d, p.MaxDelay)", "return d"},
	{"doubling one step early", "i < attempt && d > 0", "i <= attempt && d > 0"},
	{"unknown errors retried", "case !errors.Is(err, ErrTransient):", "case false:"},
	{"transient logged at ERROR", "s.deps.Logger.WarnContext(ctx, \"transient gateway failure\"", "s.deps.Logger.ErrorContext(ctx, \"transient gateway failure\""},
	{"receipt failure fails charge", "s.deps.Logger.ErrorContext(ctx, \"receipt not sent\", \"invoice_id\", inv.id, \"transaction_id\", txID, \"err\", err)", "return \"\", err"},
	{"one attempt too many", "if attempt >= s.policy.MaxAttempts {", "if attempt > s.policy.MaxAttempts {"},
	{"exhaustion drops cause", "after %d attempts: %w\", ErrPaymentFailed", "after %d attempts: %v\", ErrPaymentFailed"},
	{"cancellation ignored", "return \"\", fmt.Errorf(\"charge invoice %s: backoff interrupted: %w\", inv.id, err)", "_ = err"},
	{"idempotency key is the token", "IdempotencyKey: inv.id}", "IdempotencyKey: token}"},
	{"temp file left behind", "_ = os.Remove(tmp.Name())", "_ = tmp.Name()"},
	{"iterator error swallowed", "return 0, fmt.Errorf(\"next invoice: %w\", err)", "break"},
}

// slots bounds how many go test processes the grader runs at once.
var slots = make(chan struct{}, max(2, min(runtime.NumCPU(), 8)))

// suiteRun is the outcome of running the suite in a temporary module.
type suiteRun struct {
	exitErr     error
	ctxErr      error
	timedOut    bool
	passed      int
	failed      int
	failedTests []string
	pkgElapsed  float64 // seconds the suite package took, as reported by go test
	output      []byte
}

// killed reports whether the suite noticed the mutant: it failed a test, or it
// hung until the test binary's own timeout fired.
func (r suiteRun) killed() bool {
	return r.ctxErr == nil && r.exitErr != nil && (r.failed > 0 || r.timedOut)
}

type testEvent struct {
	Action  string
	Package string
	Test    string
	Elapsed float64
}

func TestPart1SuitePassesAgainstCorrectBilling(t *testing.T) {
	res := runSuite(t, billingSource(t))
	if res.timedOut || res.ctxErr != nil {
		t.Fatalf("suite hung against the correct billing package:\n%s", lastLines(res.output, 30))
	}
	if res.exitErr != nil || res.failed > 0 {
		t.Fatalf("suite fails against the correct billing package (failed tests: %v, passed: %d):\n%s",
			res.failedTests, res.passed, lastLines(res.output, 30))
	}
	if res.passed < minTests {
		t.Fatalf("suite has %d passing tests and subtests, want at least %d", res.passed, minTests)
	}
}

func TestPart2MutantsAreCaught(t *testing.T) {
	src := billingSource(t)
	for _, m := range mutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			mutated := applyMutant(t, src, m)
			res := runMutated(t, mutated)
			if !res.killed() {
				t.Errorf("mutant %q survived the suite (exit error: %v, failed tests: %d, timed out: %v)\n%s",
					m.name, res.exitErr, res.failed, res.timedOut, lastLines(res.output, 20))
			}
		})
	}
}

func TestPart3SuiteIsFast(t *testing.T) {
	res := runSuite(t, billingSource(t))
	if res.timedOut || res.ctxErr != nil {
		t.Fatal("suite hung against the correct billing package")
	}
	if res.exitErr != nil || res.failed > 0 {
		t.Fatalf("suite fails, so its speed is not measured (failed tests: %v)", res.failedTests)
	}
	if res.passed < minTests {
		t.Fatalf("suite has %d passing tests and subtests, want at least %d", res.passed, minTests)
	}
	if res.pkgElapsed > 10 {
		t.Fatalf("suite took %.1fs, want at most 10s", res.pkgElapsed)
	}
}

// billingSource reads the billing package the suite is graded against.
func billingSource(t *testing.T) []byte {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("billing", "billing.go"))
	if err != nil {
		t.Fatal(err)
	}
	return src
}

// suiteSource reads the suite under test: the engineer's file, or the reference
// solution when running in solutions mode.
func suiteSource(t *testing.T) []byte {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(exercise.Dir(t), "suite", "suite_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	return src
}

// applyMutant returns billing source with exactly one occurrence of m.old replaced.
func applyMutant(t *testing.T, src []byte, m mutant) []byte {
	t.Helper()
	old := []byte(m.old)
	if n := bytes.Count(src, old); n != 1 {
		t.Fatalf("mutant %q: %d occurrences of %q in billing.go, want exactly 1", m.name, n, m.old)
	}
	return bytes.Replace(src, old, []byte(m.new), 1)
}

// runSuite runs the suite against billing source in a fresh temporary module.
func runSuite(t *testing.T, billingSrc []byte) suiteRun {
	t.Helper()
	return runModule(t, buildModule(t, billingSrc, suiteSource(t)))
}

// runMutated is runSuite for a mutated billing package, throttled by slots.
func runMutated(t *testing.T, billingSrc []byte) suiteRun {
	t.Helper()
	dir := buildModule(t, billingSrc, suiteSource(t))
	slots <- struct{}{}
	defer func() { <-slots }()
	return runModule(t, dir)
}

// buildModule writes a temporary module "mut" holding billing and the suite.
func buildModule(t testing.TB, billingSrc, suiteSrc []byte) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string][]byte{
		"go.mod":                             []byte("module mut\n\ngo 1.27\n"),
		filepath.Join("billing", "billing.go"): billingSrc,
		filepath.Join("suite", "suite_test.go"): bytes.ReplaceAll(suiteSrc, []byte(billingImport), []byte("mut/billing")),
	}
	for name, data := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// runModule runs go test -json over a temporary module and parses the result.
func runModule(t *testing.T, dir string) suiteRun {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, goBinary(), "test", "-json", "-count=1", "-timeout=30s", "./...")
	cmd.Dir = dir
	cmd.Env = scratchEnv()
	out, err := cmd.CombinedOutput()

	run := suiteRun{exitErr: err, ctxErr: ctx.Err(), output: out}
	run.timedOut = bytes.Contains(out, []byte("test timed out"))
	parseEvents(out, &run)
	return run
}

func parseEvents(out []byte, run *suiteRun) {
	for line := range bytes.Lines(out) {
		var ev testEvent
		if json.Unmarshal(line, &ev) != nil {
			continue // go build messages and other non-JSON output
		}
		switch {
		case ev.Test != "" && ev.Action == "pass":
			run.passed++
		case ev.Test != "" && ev.Action == "fail":
			run.failed++
			run.failedTests = append(run.failedTests, ev.Test)
		case ev.Test == "" && strings.HasSuffix(ev.Package, "/suite") && (ev.Action == "pass" || ev.Action == "fail"):
			run.pkgElapsed = ev.Elapsed
		}
	}
}

// goBinary is the go command of the toolchain that built this test.
func goBinary() string {
	root := runtime.GOROOT()
	if root == "" {
		root = os.Getenv("GOROOT")
	}
	name := "go"
	if runtime.GOOS == "windows" {
		name = "go.exe"
	}
	return filepath.Join(root, "bin", name)
}

// scratchEnv is the environment for the temporary module: no workspace, no
// inherited flags, no toolchain switching and no network.
func scratchEnv() []string {
	drop := map[string]bool{"GOFLAGS": true, "GOWORK": true, "GOTOOLCHAIN": true, "GOPROXY": true}
	var env []string
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if !drop[strings.ToUpper(key)] {
			env = append(env, kv)
		}
	}
	return append(env, "GOFLAGS=", "GOWORK=off", "GOTOOLCHAIN=local", "GOPROXY=off")
}

// lastLines returns up to the last n lines of out, for failure messages.
func lastLines(out []byte, n int) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
