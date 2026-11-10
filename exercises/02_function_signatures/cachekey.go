package signatures

import (
	"context"
	"errors"
)

// ErrUnsupportedKeyPart is wrapped by Key for a part that cannot be encoded
// deterministically: nil, floats, and any non-scalar kind.
var ErrUnsupportedKeyPart = errors.New("unsupported cache key part")

// ErrInvalidNamespace is wrapped by Key for an empty namespace or one that
// contains ':'.
var ErrInvalidNamespace = errors.New("invalid cache namespace")

// Key builds a deterministic cache key of the form "namespace:part:part".
// Supported parts are strings, booleans, and integers of any kind, including
// named types such as `type UserID int64`. A time.Time is encoded as UTC
// RFC3339Nano, so equal instants in different zones give the same key.
// Each part is escaped so that ("a:b", "c") and ("a", "b:c") never collide.
func Key(namespace string, parts ...any) (string, error) {
	return "", nil
}

// Cache is a string-keyed in-process cache. The zero value is ready to use and
// is safe for concurrent use.
type Cache[V any] struct{}

// Get returns the value for key and whether it was present.
func (c *Cache[V]) Get(key string) (V, bool) {
	var zero V
	return zero, false
}

// Set stores v under key.
func (c *Cache[V]) Set(key string, v V) {}

// Remember returns the cached value for key. On a miss it calls load with ctx
// and caches a successful result. A load error is returned and not cached. Two
// concurrent misses may both call load, because there is no single-flight.
func (c *Cache[V]) Remember(ctx context.Context, key string, load func(context.Context) (V, error)) (V, error) {
	var zero V
	return zero, nil
}
