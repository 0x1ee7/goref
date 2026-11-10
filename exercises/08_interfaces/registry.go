package interfaces

import (
	"errors"
	"io"
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
type Registry struct{}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry { return &Registry{} }

// Register adds a factory under name. Empty names, nil factories and duplicates
// are errors; a duplicate wraps ErrDuplicateExporter.
func (r *Registry) Register(name string, f Factory) error { return nil }

// New builds the exporter registered under name. An unknown name wraps
// ErrUnknownExporter; a factory failure is wrapped with the name.
func (r *Registry) New(name string, options map[string]string) (Exporter, error) {
	return nil, errors.New("not implemented")
}

// Names returns the registered names, sorted.
func (r *Registry) Names() []string { return nil }

// Builtins returns a registry with the "csv" and "jsonl" exporters registered.
func Builtins() *Registry { return NewRegistry() }

// NewCSVExporter builds a CSV exporter. Option "delimiter" must be exactly one
// rune and defaults to ","; any other option key is an error.
func NewCSVExporter(options map[string]string) (Exporter, error) {
	return nil, errors.New("not implemented")
}

// NewJSONLinesExporter builds a JSON Lines exporter that writes one object per
// data row, keyed by the header in header order. It takes no options.
func NewJSONLinesExporter(options map[string]string) (Exporter, error) {
	return nil, errors.New("not implemented")
}
