package interfaces

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"sync"
	"unicode/utf8"
)

// ErrUnknownExporter is returned (wrapped) for a name that is not registered.
var ErrUnknownExporter = errors.New("unknown exporter")

// ErrDuplicateExporter is returned (wrapped) when a name is registered twice.
var ErrDuplicateExporter = errors.New("exporter already registered")

// Exporter writes a table to w. rows[0] is the header.
type Exporter interface {
	Export(w io.Writer, rows [][]string) error
}

// Factory builds an Exporter from string options.
type Factory func(options map[string]string) (Exporter, error)

// Registry maps names to factories. Its zero value is ready to use and it is
// safe for concurrent use (Part 7).
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry { return &Registry{} }

// Register adds a factory under name. Empty names, nil factories and duplicates
// are errors; a duplicate wraps ErrDuplicateExporter.
func (r *Registry) Register(name string, f Factory) error {
	if name == "" {
		return errors.New("register exporter: empty name")
	}
	if f == nil {
		return fmt.Errorf("register %q: nil factory", name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.factories[name]; dup {
		return fmt.Errorf("register %q: %w", name, ErrDuplicateExporter)
	}
	if r.factories == nil {
		r.factories = make(map[string]Factory)
	}
	r.factories[name] = f
	return nil
}

// New builds the exporter registered under name. An unknown name wraps
// ErrUnknownExporter; a factory failure is wrapped with the name.
func (r *Registry) New(name string, options map[string]string) (Exporter, error) {
	r.mu.RLock()
	f, ok := r.factories[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownExporter, name)
	}
	e, err := f(options)
	if err != nil {
		return nil, fmt.Errorf("exporter %q: %w", name, err)
	}
	return e, nil
}

// Names returns the registered names, sorted.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Sorted(maps.Keys(r.factories))
}

// Builtins returns a registry with the "csv" and "jsonl" exporters registered.
func Builtins() *Registry {
	r := NewRegistry()
	builtins := map[string]Factory{
		"csv":   NewCSVExporter,
		"jsonl": NewJSONLinesExporter,
	}
	for name, f := range builtins {
		if err := r.Register(name, f); err != nil {
			// Static names cannot collide or be empty; this is a programming error.
			panic(err)
		}
	}
	return r
}

// csvExporter writes RFC 4180 style CSV with a configurable delimiter.
type csvExporter struct {
	comma rune
}

// NewCSVExporter builds a CSV exporter. Option "delimiter" must be exactly one
// rune and defaults to ","; any other option key is an error.
func NewCSVExporter(options map[string]string) (Exporter, error) {
	delim := ","
	for k, v := range options {
		if k != "delimiter" {
			return nil, fmt.Errorf("csv: unknown option %q", k)
		}
		delim = v
	}
	rs := []rune(delim)
	if len(rs) != 1 || rs[0] == utf8.RuneError || rs[0] == '"' || rs[0] == '\r' || rs[0] == '\n' {
		return nil, fmt.Errorf("csv: delimiter must be one plain rune, got %q", delim)
	}
	return csvExporter{comma: rs[0]}, nil
}

// Export implements Exporter.
func (e csvExporter) Export(w io.Writer, rows [][]string) error {
	cw := csv.NewWriter(w)
	cw.Comma = e.comma
	for _, row := range rows {
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("csv: %w", err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("csv: %w", err)
	}
	return nil
}

// jsonLinesExporter writes one JSON object per data row.
type jsonLinesExporter struct{}

// NewJSONLinesExporter builds a JSON Lines exporter that writes one object per
// data row, keyed by the header in header order. It takes no options.
func NewJSONLinesExporter(options map[string]string) (Exporter, error) {
	if len(options) > 0 {
		return nil, errors.New("jsonl: takes no options")
	}
	return jsonLinesExporter{}, nil
}

// Export implements Exporter. All rows are validated before anything is
// written, so a bad row produces no partial output.
func (jsonLinesExporter) Export(w io.Writer, rows [][]string) error {
	if len(rows) == 0 {
		return nil
	}
	header := rows[0]
	var buf bytes.Buffer
	for i, row := range rows[1:] {
		if len(row) != len(header) {
			return fmt.Errorf("jsonl: row %d has %d fields, header has %d", i+1, len(row), len(header))
		}
		buf.WriteByte('{')
		for j, key := range header {
			if j > 0 {
				buf.WriteByte(',')
			}
			k, err := json.Marshal(key)
			if err != nil {
				return fmt.Errorf("jsonl: %w", err)
			}
			v, err := json.Marshal(row[j])
			if err != nil {
				return fmt.Errorf("jsonl: %w", err)
			}
			buf.Write(k)
			buf.WriteByte(':')
			buf.Write(v)
		}
		buf.WriteString("}\n")
	}
	_, err := w.Write(buf.Bytes())
	return err
}
