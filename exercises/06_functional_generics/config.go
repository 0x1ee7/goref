package functional

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
//
// Error messages (exact text is part of the spec):
//   - "line %d: expected 'key = value'"
//   - "line %d: duplicate key '%s'"
//   - "unknown key '%s'" (lexically first unknown key, checked after the scan)
//   - "missing required key 'host'" / "missing required key 'port'"
//   - "host must not be empty"
//   - "workers must be between 1 and 64"
//
// Defaults: Workers is 4; Debug is false.
func ParseConfig(text string) (Config, error) {
	return Config{}, nil
}
