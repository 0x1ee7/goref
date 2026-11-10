package functional

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// Config is the parsed service configuration.
type Config struct {
	Host    string
	Port    int
	Debug   bool
	Workers int
}

// ParseConfig parses `key = value` lines. It never panics: every problem is an
// error value. Blank lines and lines starting with '#' are ignored. Line numbers
// count every physical line, including ignored ones.
func ParseConfig(text string) (Config, error) {
	values := make(map[string]string)
	for i, line := range strings.Split(text, "\n") {
		lineNo := i + 1
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return Config{}, fmt.Errorf("line %d: expected 'key = value'", lineNo)
		}
		key = strings.TrimSpace(key)
		if _, dup := values[key]; dup {
			return Config{}, fmt.Errorf("line %d: duplicate key '%s'", lineNo, key)
		}
		values[key] = strings.TrimSpace(value)
	}

	// Sorted iteration makes "the lexically first unknown key" deterministic.
	for _, k := range slices.Sorted(maps.Keys(values)) {
		switch k {
		case "host", "port", "debug", "workers":
		default:
			return Config{}, fmt.Errorf("unknown key '%s'", k)
		}
	}

	host, ok := values["host"]
	if !ok {
		return Config{}, errors.New("missing required key 'host'")
	}
	portRaw, ok := values["port"]
	if !ok {
		return Config{}, errors.New("missing required key 'port'")
	}
	if host == "" {
		return Config{}, errors.New("host must not be empty")
	}
	port, err := ParsePort(portRaw)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{Host: host, Port: port, Workers: 4}
	if raw, ok := values["debug"]; ok {
		debug, err := ParseBool(raw)
		if err != nil {
			return Config{}, err
		}
		cfg.Debug = debug
	}
	if raw, ok := values["workers"]; ok {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 64 {
			return Config{}, errors.New("workers must be between 1 and 64")
		}
		cfg.Workers = n
	}
	return cfg, nil
}
