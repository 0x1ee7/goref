package slicesmaps

import "errors"

// DeepMerge returns a new map with override merged over base. Nested maps are
// merged recursively; every other value, including lists, is replaced by the
// override. Neither input is mutated and the result shares no maps or slices
// with either input.
func DeepMerge(base, override map[string]any) map[string]any {
	return nil
}

// ParseEnv builds a nested config map from environment variables named
// prefix + "_" + path, where path segments are separated by "__" and become
// lowercased keys. Values are coerced to bool or int when unambiguous and
// otherwise kept as strings. See ErrInvalidEnv.
func ParseEnv(environ map[string]string, prefix string) (map[string]any, error) {
	return nil, errors.New("not implemented")
}

// Layers holds the configuration sources. A nil layer is empty.
type Layers struct {
	Defaults map[string]any
	File     map[string]any
	Env      map[string]any
	CLI      map[string]any
}

// Config is an immutable, layered view over snapshots of its Layers.
type Config struct{}

// NewConfig snapshots the layers (deep copies) and returns a Config with
// precedence CLI > Env > File > Defaults.
func NewConfig(layers Layers) *Config {
	return nil
}

// Get returns the value at a dotted path such as "db.pool.size" from the
// highest-precedence layer that contains the full path. A nil value is present.
func (c *Config) Get(path string) (any, bool) {
	return nil, false
}

// Source returns the name of the layer that supplies path: "cli", "env",
// "file" or "defaults". It reports false if no layer has the path.
func (c *Config) Source(path string) (string, bool) {
	return "", false
}

// AsMap returns the fully merged effective configuration as a fresh map.
func (c *Config) AsMap() map[string]any {
	return nil
}
