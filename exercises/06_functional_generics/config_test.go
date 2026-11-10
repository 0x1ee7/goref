package functional_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	fp "goref/exercises/06_functional_generics"
)

func TestPart5Config(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    fp.Config
		wantErr string
	}{
		{
			name: "all keys",
			text: "host = example.com\nport = 8080\ndebug = true\nworkers = 8",
			want: fp.Config{Host: "example.com", Port: 8080, Debug: true, Workers: 8},
		},
		{
			name: "defaults for optional keys",
			text: "host=localhost\nport=80",
			want: fp.Config{Host: "localhost", Port: 80, Workers: 4},
		},
		{
			name: "comments and blank lines are ignored",
			text: "# c\n\nhost = a\n# x\nport = 1",
			want: fp.Config{Host: "a", Port: 1, Workers: 4},
		},
		{
			name:    "line numbers count ignored lines too",
			text:    "# header\n\nhost = a\nport = 1\n\nbogus line",
			wantErr: "line 6: expected 'key = value'",
		},
		{
			name:    "duplicate key",
			text:    "host=a\nport=1\nport=2",
			wantErr: "line 3: duplicate key 'port'",
		},
		{
			name:    "unknown key reports the lexically first one",
			text:    "host=a\nport=1\nzeta=1\nalpha=2",
			wantErr: "unknown key 'alpha'",
		},
		{
			name:    "missing host",
			text:    "port=1",
			wantErr: "missing required key 'host'",
		},
		{
			name:    "missing port",
			text:    "host=a",
			wantErr: "missing required key 'port'",
		},
		{
			name:    "empty host",
			text:    "host =\nport=1",
			wantErr: "host must not be empty",
		},
		{
			name:    "invalid port",
			text:    "host=a\nport=abc",
			wantErr: `port must be an integer, got "abc"`,
		},
		{
			name:    "invalid debug flag",
			text:    "host=a\nport=1\ndebug=maybe",
			wantErr: "expected boolean, got 'maybe'",
		},
		{
			name:    "workers zero",
			text:    "host=a\nport=1\nworkers=0",
			wantErr: "workers must be between 1 and 64",
		},
		{
			name:    "workers above limit",
			text:    "host=a\nport=1\nworkers=65",
			wantErr: "workers must be between 1 and 64",
		},
		{
			name:    "workers not a number",
			text:    "host=a\nport=1\nworkers=many",
			wantErr: "workers must be between 1 and 64",
		},
		{
			name: "workers upper bound is valid",
			text: "host=a\nport=1\nworkers=64",
			want: fp.Config{Host: "a", Port: 1, Workers: 64},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := fp.ParseConfig(tc.text)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Errorf("ParseConfig err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseConfig error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ParseConfig mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
