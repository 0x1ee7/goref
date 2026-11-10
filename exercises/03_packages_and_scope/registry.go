package scope

import "errors"

var (
	// ErrDuplicate reports a Register call for a name that is already present.
	ErrDuplicate = errors.New("duplicate registration")
	// ErrNotFound reports a Lookup for a name that was never registered.
	ErrNotFound = errors.New("not registered")
)

// Plugin is a named unit of work that transforms its input.
type Plugin func(input string) (string, error)

// Registry maps names to plugins. The zero value is an empty registry ready
// to use. A Registry is safe for concurrent use.
type Registry struct {
	// The unexported fields are yours to declare.
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds p under name. An empty name or nil p returns ErrInvalidArgument;
// a name that is already present returns ErrDuplicate. A failed Register leaves
// the registry unchanged.
func (r *Registry) Register(name string, p Plugin) error {
	return errors.New("not implemented")
}

// Lookup returns the plugin registered under name, or ErrNotFound.
func (r *Registry) Lookup(name string) (Plugin, error) {
	return nil, errors.New("not implemented")
}

// Names returns every registered name in sorted order. The slice is a copy.
func (r *Registry) Names() []string { return nil }

// Default returns the package-level registry used by Register, Lookup and Names.
func Default() *Registry { return nil }

// Register adds p to the default registry. See Registry.Register.
func Register(name string, p Plugin) error {
	return errors.New("not implemented")
}

// Lookup finds name in the default registry. See Registry.Lookup.
func Lookup(name string) (Plugin, error) {
	return nil, errors.New("not implemented")
}

// Names lists the default registry. See Registry.Names.
func Names() []string { return nil }
