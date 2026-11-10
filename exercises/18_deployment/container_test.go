package deployment_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goref/internal/exercise"
)

// instruction is one logical Dockerfile instruction (continuation lines joined).
type instruction struct {
	Op    string // upper-cased keyword, e.g. FROM, RUN, USER
	Args  string // everything after the keyword, trimmed
	Stage int    // index of the FROM stage this instruction belongs to; -1 before the first FROM
}

// parseDockerfile splits text into logical instructions. Comments and blank
// lines are dropped; a trailing backslash joins the next line.
func parseDockerfile(text string) []instruction {
	var out []instruction
	stage := -1
	emit := func(s string) {
		fields := strings.Fields(s)
		if len(fields) == 0 {
			return
		}
		op := strings.ToUpper(fields[0])
		args := strings.TrimSpace(strings.TrimPrefix(s, fields[0]))
		if op == "FROM" {
			stage++
		}
		out = append(out, instruction{Op: op, Args: args, Stage: stage})
	}

	var pending strings.Builder
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if cont, ok := strings.CutSuffix(trimmed, "\\"); ok {
			pending.WriteString(cont)
			pending.WriteString(" ")
			continue
		}
		pending.WriteString(trimmed)
		emit(pending.String())
		pending.Reset()
	}
	if pending.Len() > 0 {
		emit(pending.String())
	}
	return out
}

// imageRef returns the image of a FROM instruction, skipping flags such as --platform.
func imageRef(args string) string {
	for _, tok := range strings.Fields(args) {
		if !strings.HasPrefix(tok, "--") {
			return tok
		}
	}
	return ""
}

func fromInstructions(ins []instruction) []instruction {
	var out []instruction
	for _, in := range ins {
		if in.Op == "FROM" {
			out = append(out, in)
		}
	}
	return out
}

// lintRule is one container rule. check returns nil when the Dockerfile complies.
type lintRule struct {
	name  string
	check func(ins []instruction) error
}

var dockerfileRules = []lintRule{
	{"builder pinned to golang 1.27", func(ins []instruction) error {
		froms := fromInstructions(ins)
		if len(froms) < 2 {
			return fmt.Errorf("need a builder stage and a runtime stage, found %d FROM", len(froms))
		}
		ref := imageRef(froms[0].Args)
		if !strings.HasPrefix(ref, "golang:1.27") {
			return fmt.Errorf("builder image %q; want golang:1.27 (pinned to the go.mod toolchain)", ref)
		}
		return nil
	}},
	{"every image tagged or digest-pinned, none latest", func(ins []instruction) error {
		for _, f := range fromInstructions(ins) {
			ref := imageRef(f.Args)
			if ref == "scratch" || strings.Contains(ref, "@sha256:") {
				continue
			}
			tagAt := strings.LastIndex(ref, ":")
			if tagAt <= strings.LastIndex(ref, "/") || strings.HasSuffix(ref, ":latest") {
				return fmt.Errorf("image %q has no explicit non-latest tag", ref)
			}
		}
		return nil
	}},
	{"runtime is distroless static nonroot or scratch", func(ins []instruction) error {
		froms := fromInstructions(ins)
		if len(froms) == 0 {
			return fmt.Errorf("no FROM instructions")
		}
		ref := imageRef(froms[len(froms)-1].Args)
		if ref == "scratch" || (strings.HasPrefix(ref, "gcr.io/distroless/static") && strings.HasSuffix(ref, ":nonroot")) {
			return nil
		}
		return fmt.Errorf("runtime image %q; want gcr.io/distroless/static-debian12:nonroot or scratch", ref)
	}},
	{"static build with CGO_ENABLED=0 in the builder", func(ins []instruction) error {
		last := len(fromInstructions(ins)) - 1
		for _, in := range ins {
			if in.Stage < last && (in.Op == "RUN" || in.Op == "ENV") && strings.Contains(in.Args, "CGO_ENABLED=0") {
				return nil
			}
		}
		return fmt.Errorf("no CGO_ENABLED=0 in the builder stage")
	}},
	{"binary copied from the builder stage", func(ins []instruction) error {
		last := len(fromInstructions(ins)) - 1
		for _, in := range ins {
			if in.Stage == last && in.Op == "COPY" && strings.Contains(in.Args, "--from=") {
				return nil
			}
		}
		return fmt.Errorf("runtime stage has no COPY --from= of the built binary")
	}},
	{"final stage runs as non-root USER", func(ins []instruction) error {
		last := len(fromInstructions(ins)) - 1
		user := ""
		for _, in := range ins {
			if in.Stage == last && in.Op == "USER" {
				user = strings.Split(strings.Fields(in.Args)[0], ":")[0]
			}
		}
		if user == "" {
			return fmt.Errorf("final stage sets no USER")
		}
		if user == "root" || user == "0" {
			return fmt.Errorf("final stage runs as %q", user)
		}
		return nil
	}},
	{"ENTRYPOINT present in the final stage", func(ins []instruction) error {
		last := len(fromInstructions(ins)) - 1
		for _, in := range ins {
			if in.Stage == last && in.Op == "ENTRYPOINT" {
				return nil
			}
		}
		return fmt.Errorf("final stage has no ENTRYPOINT")
	}},
	{"ENTRYPOINT and CMD use exec form", func(ins []instruction) error {
		for _, in := range ins {
			if in.Op != "ENTRYPOINT" && in.Op != "CMD" {
				continue
			}
			var argv []string
			if !strings.HasPrefix(in.Args, "[") || json.Unmarshal([]byte(in.Args), &argv) != nil || len(argv) == 0 {
				return fmt.Errorf("%s %s is shell form; use the JSON array form", in.Op, in.Args)
			}
		}
		return nil
	}},
}

// dockerignoreRules checks the .dockerignore text.
func dockerignoreRules(text string) error {
	lines := map[string]bool{}
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "#") {
			lines[l] = true
		}
	}
	if !lines[".git"] && !lines[".git/"] {
		return fmt.Errorf(".dockerignore does not exclude .git")
	}
	if !lines["bin"] && !lines["bin/"] && !lines["dist"] && !lines["dist/"] {
		return fmt.Errorf(".dockerignore does not exclude build output (bin/ or dist/)")
	}
	return nil
}

func readExerciseFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(exercise.Dir(t), name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(data)
}

func TestPart6ContainerLint(t *testing.T) {
	dockerfile := readExerciseFile(t, "Dockerfile")
	if strings.TrimSpace(dockerfile) == "" {
		t.Fatal("Dockerfile is empty; write the multi-stage image definition")
	}
	ignore := readExerciseFile(t, ".dockerignore")
	if strings.TrimSpace(ignore) == "" {
		t.Fatal(".dockerignore is empty")
	}
	ins := parseDockerfile(dockerfile)

	for _, rule := range dockerfileRules {
		t.Run(rule.name, func(t *testing.T) {
			if err := rule.check(ins); err != nil {
				t.Error(err)
			}
		})
	}

	t.Run("dockerignore excludes .git and build output", func(t *testing.T) {
		if err := dockerignoreRules(ignore); err != nil {
			t.Error(err)
		}
	})

	// The linter itself must reject a bad image and accept a good one, or the rules above prove nothing.
	t.Run("lint self-check", func(t *testing.T) {
		const bad = `FROM golang:latest AS build
RUN go build -o /out/app .
FROM alpine
COPY --from=build /out/app /app
USER root
CMD /app serve
`
		badIns := parseDockerfile(bad)
		for _, rule := range dockerfileRules {
			switch rule.name {
			case "builder pinned to golang 1.27",
				"every image tagged or digest-pinned, none latest",
				"runtime is distroless static nonroot or scratch",
				"final stage runs as non-root USER",
				"ENTRYPOINT present in the final stage",
				"ENTRYPOINT and CMD use exec form":
				if rule.check(badIns) == nil {
					t.Errorf("rule %q accepted a bad Dockerfile", rule.name)
				}
			}
		}
		if err := dockerignoreRules("bin/\n"); err == nil {
			t.Error(".dockerignore rule accepted a file without .git")
		}
		const good = `# comment
FROM golang:1.27.4 AS build
RUN CGO_ENABLED=0 go build -trimpath -o /out/app \
    ./cmd/app
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
USER 65532:65532
ENTRYPOINT ["/app"]
CMD ["serve"]
`
		for _, rule := range dockerfileRules {
			if err := rule.check(parseDockerfile(good)); err != nil {
				t.Errorf("rule %q rejected a good Dockerfile: %v", rule.name, err)
			}
		}
	})
}
