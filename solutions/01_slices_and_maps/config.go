package slicesmaps

import (
	"fmt"
	"strconv"
	"strings"
)

// cloneValue deep-copies the map and slice shapes that a config tree can hold.
// Scalars are immutable and returned as-is.
func cloneValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		return cloneMap(x)
	case []any:
		if x == nil {
			return x
		}
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = cloneValue(e)
		}
		return out
	default:
		return v
	}
}

// cloneMap deep-copies m. A nil map stays nil.
func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = cloneValue(v)
	}
	return out
}

// DeepMerge returns a new map with override merged over base. Nested maps are
// merged recursively; every other value, including lists, is replaced by the
// override. Neither input is mutated and the result shares no maps or slices
// with either input.
func DeepMerge(base, override map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(override))
	for k, v := range base {
		out[k] = cloneValue(v)
	}
	for k, ov := range override {
		bm, bIsMap := out[k].(map[string]any)
		om, oIsMap := ov.(map[string]any)
		if bIsMap && oIsMap {
			out[k] = DeepMerge(bm, om)
			continue
		}
		out[k] = cloneValue(ov)
	}
	return out
}

// ParseEnv builds a nested config map from environment variables named
// prefix + "_" + path. Path segments are separated by "__" and lowercased, so
// APP_DB__POOL__SIZE=10 becomes {"db": {"pool": {"size": 10}}}. Values are
// coerced to bool or int when unambiguous and otherwise kept as strings. Empty
// segments and any key that conflicts with another key yield ErrInvalidEnv.
// The result does not depend on map iteration order.
func ParseEnv(environ map[string]string, prefix string) (map[string]any, error) {
	root := make(map[string]any)
	want := prefix + "_"
	for key, raw := range environ {
		if !strings.HasPrefix(key, want) {
			continue
		}
		segments := strings.Split(strings.ToLower(key[len(want):]), "__")
		for _, seg := range segments {
			if seg == "" {
				return nil, fmt.Errorf("%w: %s has an empty path segment", ErrInvalidEnv, key)
			}
		}
		if err := insertPath(root, segments, coerce(raw), key); err != nil {
			return nil, err
		}
	}
	return root, nil
}

// insertPath stores value at path inside root, creating intermediate maps. It
// rejects a leaf that already exists and a path that runs through a scalar, so
// conflicting keys fail whichever order they are visited in.
func insertPath(root map[string]any, path []string, value any, key string) error {
	node := root
	for _, seg := range path[:len(path)-1] {
		existing, ok := node[seg]
		if !ok {
			child := make(map[string]any)
			node[seg] = child
			node = child
			continue
		}
		child, isMap := existing.(map[string]any)
		if !isMap {
			return fmt.Errorf("%w: %s conflicts with a value at a parent key", ErrInvalidEnv, key)
		}
		node = child
	}
	leaf := path[len(path)-1]
	if _, exists := node[leaf]; exists {
		return fmt.Errorf("%w: %s conflicts with another key", ErrInvalidEnv, key)
	}
	node[leaf] = value
	return nil
}

// coerce converts an environment string to bool or int when unambiguous. Only
// an optional '-' followed by ASCII digits becomes an int, and only if it fits.
func coerce(s string) any {
	switch {
	case strings.EqualFold(s, "true"):
		return true
	case strings.EqualFold(s, "false"):
		return false
	case isDecimalInt(s):
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return s
}

// isDecimalInt reports whether s is an optional '-' followed by one or more ASCII digits.
func isDecimalInt(s string) bool {
	digits := strings.TrimPrefix(s, "-")
	if digits == "" {
		return false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Layers holds the configuration sources. A nil layer is empty.
type Layers struct {
	Defaults map[string]any
	File     map[string]any
	Env      map[string]any
	CLI      map[string]any
}

// namedLayer pairs a snapshot with the name reported by Source.
type namedLayer struct {
	name string
	data map[string]any
}

// Config is an immutable, layered view over snapshots of its Layers.
type Config struct {
	layers []namedLayer // highest precedence first
}

// NewConfig snapshots the layers (deep copies) and returns a Config with
// precedence CLI > Env > File > Defaults.
func NewConfig(layers Layers) *Config {
	return &Config{layers: []namedLayer{
		{"cli", cloneMap(layers.CLI)},
		{"env", cloneMap(layers.Env)},
		{"file", cloneMap(layers.File)},
		{"defaults", cloneMap(layers.Defaults)},
	}}
}

// Get returns the value at a dotted path such as "db.pool.size" from the
// highest-precedence layer that contains the full path. A nil value is present.
// The returned value is a copy.
func (c *Config) Get(path string) (any, bool) {
	for _, layer := range c.layers {
		if v, ok := lookup(layer.data, path); ok {
			return cloneValue(v), true
		}
	}
	return nil, false
}

// Source returns the name of the layer that supplies path: "cli", "env",
// "file" or "defaults". It reports false if no layer has the path.
func (c *Config) Source(path string) (string, bool) {
	for _, layer := range c.layers {
		if _, ok := lookup(layer.data, path); ok {
			return layer.name, true
		}
	}
	return "", false
}

// AsMap returns the fully merged effective configuration as a fresh map.
func (c *Config) AsMap() map[string]any {
	out := make(map[string]any)
	for i := len(c.layers) - 1; i >= 0; i-- {
		out = DeepMerge(out, c.layers[i].data)
	}
	return out
}

// lookup resolves a dotted path in m. A segment that passes through a
// non-mapping value makes the path absent.
func lookup(m map[string]any, path string) (any, bool) {
	segments := strings.Split(path, ".")
	node := m
	for i, seg := range segments {
		v, ok := node[seg]
		if !ok {
			return nil, false
		}
		if i == len(segments)-1 {
			return v, true
		}
		next, isMap := v.(map[string]any)
		if !isMap {
			return nil, false
		}
		node = next
	}
	return nil, false
}
