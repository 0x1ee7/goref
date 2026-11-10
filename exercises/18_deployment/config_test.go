package deployment_test

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	deployment "goref/exercises/18_deployment"
)

const validToken = "0123456789abcdef-token"

// baseEnv is a complete, valid environment. Tests copy it and override keys.
func baseEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL": "postgresql://app:hunter2@db.internal:5432/orders",
		"API_TOKEN":    validToken,
	}
}

func getenvFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// withEnv returns baseEnv with the given overrides applied.
func withEnv(overrides map[string]string) map[string]string {
	env := baseEnv()
	for k, v := range overrides {
		env[k] = v
	}
	return env
}

// fieldErrors collects the *FieldError values inside an errors.Join result, keyed by variable.
func fieldErrors(t *testing.T, err error) map[string]*deployment.FieldError {
	t.Helper()
	out := map[string]*deployment.FieldError{}
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("error %v is not an errors.Join result", err)
	}
	for _, e := range joined.Unwrap() {
		var fe *deployment.FieldError
		if !errors.As(e, &fe) {
			t.Fatalf("joined error %v is not a *FieldError", e)
		}
		out[fe.Var] = fe
	}
	return out
}

func TestPart1DefaultsFromRequiredOnly(t *testing.T) {
	cfg, err := deployment.LoadConfig(getenvFrom(baseEnv()))
	if err != nil {
		t.Fatalf("LoadConfig: unexpected error: %v", err)
	}
	want := deployment.Config{
		DatabaseURL:    "postgresql://app:hunter2@db.internal:5432/orders",
		APIToken:       validToken,
		AppName:        "goref-service",
		Port:           8080,
		Debug:          false,
		AllowedHosts:   nil,
		RequestTimeout: 30 * time.Second,
		LogLevel:       slog.LevelInfo,
	}
	if diff := cmp.Diff(want, cfg); diff != "" {
		t.Errorf("LoadConfig defaults (-want +got):\n%s", diff)
	}
}

func TestPart1ParsesOverrides(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want func(*deployment.Config)
	}{
		{
			name: "port",
			env:  map[string]string{"PORT": "9000"},
			want: func(c *deployment.Config) { c.Port = 9000 },
		},
		{
			name: "app name trimmed",
			env:  map[string]string{"APP_NAME": "  billing  "},
			want: func(c *deployment.Config) { c.AppName = "billing" },
		},
		{
			name: "debug true",
			env:  map[string]string{"DEBUG": "true"},
			want: func(c *deployment.Config) { c.Debug = true },
		},
		{
			name: "debug one",
			env:  map[string]string{"DEBUG": "1"},
			want: func(c *deployment.Config) { c.Debug = true },
		},
		{
			name: "debug false",
			env:  map[string]string{"DEBUG": "false"},
			want: func(c *deployment.Config) { c.Debug = false },
		},
		{
			name: "allowed hosts drop blanks and spaces",
			env:  map[string]string{"ALLOWED_HOSTS": "a.example, b.example ,,"},
			want: func(c *deployment.Config) { c.AllowedHosts = []string{"a.example", "b.example"} },
		},
		{
			name: "request timeout duration",
			env:  map[string]string{"REQUEST_TIMEOUT": "5s"},
			want: func(c *deployment.Config) { c.RequestTimeout = 5 * time.Second },
		},
		{
			name: "request timeout bare seconds",
			env:  map[string]string{"REQUEST_TIMEOUT": "45"},
			want: func(c *deployment.Config) { c.RequestTimeout = 45 * time.Second },
		},
		{
			name: "log level case-insensitive",
			env:  map[string]string{"LOG_LEVEL": "debug"},
			want: func(c *deployment.Config) { c.LogLevel = slog.LevelDebug },
		},
		{
			name: "blank counts as unset",
			env:  map[string]string{"PORT": "   ", "APP_NAME": ""},
			want: func(c *deployment.Config) {}, // defaults stay
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := deployment.LoadConfig(getenvFrom(withEnv(tt.env)))
			if err != nil {
				t.Fatalf("LoadConfig: unexpected error: %v", err)
			}
			want := deployment.Config{
				DatabaseURL:    "postgresql://app:hunter2@db.internal:5432/orders",
				APIToken:       validToken,
				AppName:        "goref-service",
				Port:           8080,
				RequestTimeout: 30 * time.Second,
				LogLevel:       slog.LevelInfo,
			}
			tt.want(&want)
			if diff := cmp.Diff(want, cfg); diff != "" {
				t.Errorf("LoadConfig (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPart1RequiredVariables(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want []string // variables that must be reported
	}{
		{"missing database url", map[string]string{"DATABASE_URL": ""}, []string{"DATABASE_URL"}},
		{"blank database url", map[string]string{"DATABASE_URL": "   "}, []string{"DATABASE_URL"}},
		{"missing api token", map[string]string{"API_TOKEN": ""}, []string{"API_TOKEN"}},
		{"both missing aggregated", map[string]string{"DATABASE_URL": "", "API_TOKEN": ""}, []string{"DATABASE_URL", "API_TOKEN"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := deployment.LoadConfig(getenvFrom(withEnv(tt.env)))
			if err == nil {
				t.Fatal("LoadConfig: want an error for missing required variables, got nil")
			}
			fes := fieldErrors(t, err)
			for _, v := range tt.want {
				fe, ok := fes[v]
				if !ok {
					t.Errorf("no FieldError for %s; got %v", v, err)
					continue
				}
				if !errors.Is(fe, deployment.ErrRequired) {
					t.Errorf("%s: errors.Is(err, ErrRequired) = false; err = %v", v, fe)
				}
			}
			if len(fes) != len(tt.want) {
				t.Errorf("got %d field errors, want %d: %v", len(fes), len(tt.want), err)
			}
		})
	}
}

func TestPart1InvalidValues(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		v    string // variable expected in the FieldError
	}{
		{"port not a number", map[string]string{"PORT": "abc"}, "PORT"},
		{"port with sign", map[string]string{"PORT": "+80"}, "PORT"},
		{"port zero", map[string]string{"PORT": "0"}, "PORT"},
		{"port too large", map[string]string{"PORT": "70000"}, "PORT"},
		{"timeout not a duration", map[string]string{"REQUEST_TIMEOUT": "soon"}, "REQUEST_TIMEOUT"},
		{"timeout negative", map[string]string{"REQUEST_TIMEOUT": "-1s"}, "REQUEST_TIMEOUT"},
		{"timeout zero", map[string]string{"REQUEST_TIMEOUT": "0s"}, "REQUEST_TIMEOUT"},
		{"debug not a bool", map[string]string{"DEBUG": "maybe"}, "DEBUG"},
		{"log level unknown", map[string]string{"LOG_LEVEL": "TRACE"}, "LOG_LEVEL"},
		{"token too short", map[string]string{"API_TOKEN": "short"}, "API_TOKEN"},
		{"database scheme unsupported", map[string]string{"DATABASE_URL": "mysql://u:p@h/db"}, "DATABASE_URL"},
		{"database url unparseable", map[string]string{"DATABASE_URL": "://nope"}, "DATABASE_URL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := deployment.LoadConfig(getenvFrom(withEnv(tt.env)))
			if err == nil {
				t.Fatalf("LoadConfig(%v): want error, got nil", tt.env)
			}
			fe, ok := fieldErrors(t, err)[tt.v]
			if !ok {
				t.Fatalf("no FieldError for %s; got %v", tt.v, err)
			}
			if !errors.Is(fe, deployment.ErrInvalid) {
				t.Errorf("errors.Is(%s error, ErrInvalid) = false; err = %v", tt.v, fe)
			}
		})
	}
}

func TestPart1AggregatesAllProblems(t *testing.T) {
	env := withEnv(map[string]string{
		"DATABASE_URL":    "",
		"PORT":            "x",
		"REQUEST_TIMEOUT": "later",
		"LOG_LEVEL":       "loud",
	})
	_, err := deployment.LoadConfig(getenvFrom(env))
	if err == nil {
		t.Fatal("LoadConfig: want aggregated error, got nil")
	}
	got := fieldErrors(t, err)
	for _, v := range []string{"DATABASE_URL", "PORT", "REQUEST_TIMEOUT", "LOG_LEVEL"} {
		if _, ok := got[v]; !ok {
			t.Errorf("missing FieldError for %s; all problems must be reported together: %v", v, err)
		}
	}
	if len(got) != 4 {
		t.Errorf("got %d field errors, want 4: %v", len(got), err)
	}
}

func TestPart1ErrorTextNamesVariable(t *testing.T) {
	_, err := deployment.LoadConfig(getenvFrom(withEnv(map[string]string{"PORT": "x"})))
	if err == nil {
		t.Fatal("LoadConfig: want error for PORT=x, got nil")
	}
	fe := fieldErrors(t, err)["PORT"]
	if fe == nil {
		t.Fatalf("no PORT FieldError: %v", err)
	}
	if !strings.HasPrefix(fe.Error(), "PORT: ") {
		t.Errorf("FieldError.Error() = %q, want prefix %q", fe.Error(), "PORT: ")
	}
	if !strings.Contains(err.Error(), "PORT: ") {
		t.Errorf("joined error text %q does not name PORT", err.Error())
	}
}

func TestPart1SecretsAreMasked(t *testing.T) {
	const password = "hunter2"
	env := baseEnv()
	env["API_TOKEN"] = "supersecret-token-12345"
	env["DATABASE_URL"] = "postgresql://app:" + password + "@db.internal:5432/orders"
	cfg, err := deployment.LoadConfig(getenvFrom(env))
	if err != nil {
		t.Fatalf("LoadConfig: unexpected error: %v", err)
	}

	if got := string(cfg.APIToken); got != "supersecret-token-12345" {
		t.Errorf("string(cfg.APIToken) = %q, want the real value", got)
	}

	var logBuf bytes.Buffer
	slog.New(slog.NewJSONHandler(&logBuf, nil)).Info("loaded", "cfg", cfg)

	outputs := map[string]string{
		"String()":      cfg.String(),
		"%v":            fmt.Sprintf("%v", cfg),
		"%+v":           fmt.Sprintf("%+v", cfg),
		"%#v":           fmt.Sprintf("%#v", cfg),
		"%v token":      fmt.Sprintf("%v", cfg.APIToken),
		"%#v token":     fmt.Sprintf("%#v", cfg.APIToken),
		"slog JSON log": logBuf.String(),
	}
	for name, out := range outputs {
		if strings.Contains(out, "supersecret-token-12345") {
			t.Errorf("%s leaks the API token: %s", name, out)
		}
		if strings.Contains(out, password) {
			t.Errorf("%s leaks the database password: %s", name, out)
		}
	}
	if !strings.Contains(outputs["String()"], "app:***@") {
		t.Errorf("String() = %q, want the database password replaced by ***", outputs["String()"])
	}
}

func TestPart1RedactURL(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"password masked", "postgresql://app:hunter2@db:5432/x", "postgresql://app:***@db:5432/x"},
		{"no password unchanged", "postgresql://db:5432/x", "postgresql://db:5432/x"},
		{"sqlite path unchanged", "sqlite:///tmp/orders.db", "sqlite:///tmp/orders.db"},
		{"unparseable unchanged", "://nope", "://nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deployment.RedactURL(tt.in); got != tt.want {
				t.Errorf("RedactURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
