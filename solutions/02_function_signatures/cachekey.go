package signatures

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrUnsupportedKeyPart reports a key part whose type has no canonical text form.
var ErrUnsupportedKeyPart = errors.New("unsupported cache key part")

// ErrInvalidNamespace reports an empty namespace or one containing ':'.
var ErrInvalidNamespace = errors.New("invalid cache key namespace")

// partEscaper encodes the separator and the escape character itself, so no
// two different part lists can render to the same key.
var partEscaper = strings.NewReplacer("%", "%25", ":", "%3A")

// Key renders namespace and parts as "namespace:part:part". Supported parts are
// strings, bools, time.Time (rendered in UTC as RFC 3339 with nanoseconds), and
// every integer kind, including named types, selected by reflect.Kind. Any
// other part, including nil and floats, is an error.
func Key(namespace string, parts ...any) (string, error) {
	if namespace == "" || strings.Contains(namespace, ":") {
		return "", fmt.Errorf("%w: %q", ErrInvalidNamespace, namespace)
	}
	var b strings.Builder
	b.WriteString(namespace)
	for _, p := range parts {
		s, err := encodePart(p)
		if err != nil {
			return "", err
		}
		b.WriteByte(':')
		b.WriteString(s)
	}
	return b.String(), nil
}

func encodePart(p any) (string, error) {
	switch v := p.(type) {
	case string:
		return partEscaper.Replace(v), nil
	case bool:
		return strconv.FormatBool(v), nil
	case time.Time:
		return v.UTC().Format(time.RFC3339Nano), nil
	}

	rv := reflect.ValueOf(p)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(rv.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(rv.Uint(), 10), nil
	case reflect.String:
		return partEscaper.Replace(rv.String()), nil
	case reflect.Bool:
		return strconv.FormatBool(rv.Bool()), nil
	}
	return "", fmt.Errorf("%w: %T", ErrUnsupportedKeyPart, p)
}

// Cache is a concurrency-safe in-memory map. The zero value is ready to use.
type Cache[V any] struct {
	mu sync.RWMutex
	m  map[string]V
}

// Get returns the value stored under key and whether it was present.
func (c *Cache[V]) Get(key string) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.m[key]
	return v, ok
}

// Set stores v under key, replacing any previous value.
func (c *Cache[V]) Set(key string, v V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = make(map[string]V)
	}
	c.m[key] = v
}

// Remember returns the cached value for key. On a miss it calls load, caches a
// successful result, and returns it. Errors are returned and not cached. There
// is no single-flight, so concurrent misses may each call load.
func (c *Cache[V]) Remember(ctx context.Context, key string, load func(context.Context) (V, error)) (V, error) {
	if v, ok := c.Get(key); ok {
		return v, nil
	}
	v, err := load(ctx)
	if err != nil {
		var zero V
		return zero, err
	}
	c.Set(key, v)
	return v, nil
}
