package scope

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
)

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
	mu      sync.RWMutex
	plugins map[string]Plugin
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds p under name. An empty name or nil p returns ErrInvalidArgument;
// a name that is already present returns ErrDuplicate. A failed Register leaves
// the registry unchanged.
func (r *Registry) Register(name string, p Plugin) error {
	if name == "" || p == nil {
		return fmt.Errorf("%w: name %q, plugin nil=%t", ErrInvalidArgument, name, p == nil)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.plugins[name]; ok {
		return fmt.Errorf("%w: %q", ErrDuplicate, name)
	}
	if r.plugins == nil {
		r.plugins = make(map[string]Plugin)
	}
	r.plugins[name] = p
	return nil
}

// Lookup returns the plugin registered under name, or ErrNotFound.
func (r *Registry) Lookup(name string) (Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	return p, nil
}

// Names returns every registered name in sorted order. The slice is a copy.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Sorted(maps.Keys(r.plugins))
}

var defaultRegistry Registry

// Default returns the package-level registry used by Register, Lookup and Names.
func Default() *Registry { return &defaultRegistry }

// Register adds p to the default registry. See Registry.Register.
func Register(name string, p Plugin) error {
	return defaultRegistry.Register(name, p)
}

// Lookup finds name in the default registry. See Registry.Lookup.
func Lookup(name string) (Plugin, error) {
	return defaultRegistry.Lookup(name)
}

// Names lists the default registry. See Registry.Names.
func Names() []string {
	return defaultRegistry.Names()
}
