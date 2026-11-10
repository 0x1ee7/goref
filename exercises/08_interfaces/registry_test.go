package interfaces_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"goref/exercises/08_interfaces"
)

var tableRows = [][]string{{"name", "note"}, {"ann", "hi, there"}}

// upperExporter is a user-defined Exporter registered at runtime.
type upperExporter struct{}

func (upperExporter) Export(w io.Writer, rows [][]string) error {
	for _, row := range rows {
		if _, err := io.WriteString(w, strings.ToUpper(strings.Join(row, "|"))+"\n"); err != nil {
			return err
		}
	}
	return nil
}

func upperFactory(map[string]string) (interfaces.Exporter, error) { return upperExporter{}, nil }

func exportString(t *testing.T, e interfaces.Exporter, rows [][]string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := e.Export(&buf, rows); err != nil {
		t.Fatalf("Export: %v", err)
	}
	return buf.String()
}

func TestPart7BuiltinExporters(t *testing.T) {
	t.Run("csv quotes values containing the delimiter", func(t *testing.T) {
		e, err := interfaces.Builtins().New("csv", nil)
		if err != nil {
			t.Fatalf("New(csv): %v", err)
		}
		want := "name,note\nann,\"hi, there\"\n"
		if got := exportString(t, e, tableRows); got != want {
			t.Errorf("csv output = %q, want %q", got, want)
		}
	})

	t.Run("csv delimiter option", func(t *testing.T) {
		e, err := interfaces.Builtins().New("csv", map[string]string{"delimiter": ";"})
		if err != nil {
			t.Fatalf("New(csv, ;): %v", err)
		}
		if got, want := exportString(t, e, [][]string{{"a", "b"}, {"1", "2"}}), "a;b\n1;2\n"; got != want {
			t.Errorf("csv output = %q, want %q", got, want)
		}
		if _, err := interfaces.Builtins().New("csv", map[string]string{"delimiter": "::"}); err == nil {
			t.Error("multi-rune delimiter: err = nil, want error")
		}
	})

	t.Run("unknown option keys are rejected", func(t *testing.T) {
		if _, err := interfaces.Builtins().New("csv", map[string]string{"delimiter": ","}); err != nil {
			t.Fatalf("sanity: valid csv options rejected: %v", err)
		}
		if _, err := interfaces.Builtins().New("csv", map[string]string{"quote": "'"}); err == nil {
			t.Error("unknown csv option: err = nil, want error")
		}
	})

	t.Run("jsonl writes one object per row in header order", func(t *testing.T) {
		e, err := interfaces.Builtins().New("jsonl", nil)
		if err != nil {
			t.Fatalf("New(jsonl): %v", err)
		}
		rows := [][]string{{"name", "age"}, {"ann", "3"}, {"bob", "41"}}
		want := "{\"name\":\"ann\",\"age\":\"3\"}\n{\"name\":\"bob\",\"age\":\"41\"}\n"
		if got := exportString(t, e, rows); got != want {
			t.Errorf("jsonl output = %q, want %q", got, want)
		}
	})

	t.Run("jsonl rejects rows that do not match the header", func(t *testing.T) {
		e, err := interfaces.Builtins().New("jsonl", nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.Export(io.Discard, [][]string{{"a", "b"}, {"1", "2"}}); err != nil {
			t.Fatalf("sanity: matching row rejected: %v", err)
		}
		if err := e.Export(io.Discard, [][]string{{"a", "b"}, {"1"}}); err == nil {
			t.Error("row with too few columns: err = nil, want error")
		}
	})

	t.Run("Builtins lists csv and jsonl", func(t *testing.T) {
		if diff := cmp.Diff([]string{"csv", "jsonl"}, interfaces.Builtins().Names()); diff != "" {
			t.Errorf("Names (-want +got):\n%s", diff)
		}
	})
}

func TestPart7Registry(t *testing.T) {
	t.Run("register and build a custom exporter", func(t *testing.T) {
		reg := interfaces.NewRegistry()
		if err := reg.Register("upper", upperFactory); err != nil {
			t.Fatalf("Register: %v", err)
		}
		e, err := reg.New("upper", nil)
		if err != nil {
			t.Fatalf("New(upper): %v", err)
		}
		if got, want := exportString(t, e, [][]string{{"a", "b"}}), "A|B\n"; got != want {
			t.Errorf("output = %q, want %q", got, want)
		}
	})

	t.Run("unknown name wraps ErrUnknownExporter", func(t *testing.T) {
		if _, err := interfaces.Builtins().New("csv", nil); err != nil {
			t.Fatalf("sanity: csv not built: %v", err)
		}
		_, err := interfaces.Builtins().New("xml", nil)
		if !errors.Is(err, interfaces.ErrUnknownExporter) {
			t.Errorf("New(xml) err = %v, want ErrUnknownExporter", err)
		}
	})

	t.Run("duplicate, empty and nil registrations are rejected", func(t *testing.T) {
		reg := interfaces.Builtins()
		if err := reg.Register("ok", upperFactory); err != nil {
			t.Fatalf("sanity: fresh name rejected: %v", err)
		}
		if err := reg.Register("csv", upperFactory); !errors.Is(err, interfaces.ErrDuplicateExporter) {
			t.Errorf("duplicate Register err = %v, want ErrDuplicateExporter", err)
		}
		if err := reg.Register("", upperFactory); err == nil {
			t.Error("empty name: err = nil, want error")
		}
		if err := reg.Register("nilf", nil); err == nil {
			t.Error("nil factory: err = nil, want error")
		}
	})

	t.Run("Names are sorted", func(t *testing.T) {
		reg := interfaces.NewRegistry()
		for _, name := range []string{"zeta", "alpha", "mid"} {
			if err := reg.Register(name, upperFactory); err != nil {
				t.Fatalf("Register(%s): %v", name, err)
			}
		}
		if diff := cmp.Diff([]string{"alpha", "mid", "zeta"}, reg.Names()); diff != "" {
			t.Errorf("Names (-want +got):\n%s", diff)
		}
	})

	t.Run("factory failure is wrapped with the name", func(t *testing.T) {
		errBroken := errors.New("bad config")
		reg := interfaces.NewRegistry()
		if err := reg.Register("broken", func(map[string]string) (interfaces.Exporter, error) {
			return nil, errBroken
		}); err != nil {
			t.Fatalf("Register: %v", err)
		}
		_, err := reg.New("broken", nil)
		if !errors.Is(err, errBroken) {
			t.Fatalf("New(broken) err = %v, want wrapped errBroken", err)
		}
		if !strings.Contains(err.Error(), "broken") {
			t.Errorf("error %q does not name the exporter", err.Error())
		}
	})

	t.Run("registries are isolated from each other", func(t *testing.T) {
		a, b := interfaces.NewRegistry(), interfaces.NewRegistry()
		if err := a.Register("only-a", upperFactory); err != nil {
			t.Fatal(err)
		}
		if _, err := a.New("only-a", nil); err != nil {
			t.Fatalf("sanity: registered name not built: %v", err)
		}
		if _, err := b.New("only-a", nil); !errors.Is(err, interfaces.ErrUnknownExporter) {
			t.Errorf("other registry New err = %v, want ErrUnknownExporter", err)
		}
	})
}
