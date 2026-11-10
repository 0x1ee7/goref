package perf

import (
	"slices"
	"strings"
	"unsafe"
)

// Param is one key/value pair of a URL query string.
type Param struct {
	Key   string
	Value string
}

// CSVRecord encodes fields as one RFC 4180 record, without a trailing newline.
// A field is quoted only if it contains a comma, double quote, CR or LF; embedded
// quotes are doubled. CSVRecord performs exactly one allocation for any non-empty
// input (the returned string).
//
// The encoder computes the exact output length first, then fills one buffer
// that is handed to the caller as a string without a copy.
func CSVRecord(fields []string) string {
	n := csvLen(fields)
	if n == 0 {
		return ""
	}
	buf := make([]byte, 0, n)
	buf = appendCSVRecord(buf, fields)
	return unsafe.String(unsafe.SliceData(buf), len(buf))
}

// AppendCSVRecord appends the CSVRecord encoding of fields to dst and returns the
// extended slice. It grows dst at most once, and not at all when dst already has
// enough spare capacity.
func AppendCSVRecord(dst []byte, fields []string) []byte {
	dst = slices.Grow(dst, csvLen(fields))
	return appendCSVRecord(dst, fields)
}

// QueryString encodes params as an application/x-www-form-urlencoded query
// string, in the order given, joined with '&'. Keys and values use the escaping
// of url.QueryEscape. QueryString performs exactly one allocation for any
// non-empty input.
func QueryString(params []Param) string {
	n := queryLen(params)
	if n == 0 {
		return ""
	}
	buf := make([]byte, 0, n)
	for i, p := range params {
		if i > 0 {
			buf = append(buf, '&')
		}
		buf = appendEscaped(buf, p.Key)
		buf = append(buf, '=')
		buf = appendEscaped(buf, p.Value)
	}
	return unsafe.String(unsafe.SliceData(buf), len(buf))
}

// needsCSVQuote reports whether s must be wrapped in double quotes.
func needsCSVQuote(s string) bool {
	return strings.ContainsAny(s, ",\"\r\n")
}

// csvLen returns the exact encoded length of fields, including separators.
func csvLen(fields []string) int {
	n := 0
	for _, f := range fields {
		if needsCSVQuote(f) {
			n += len(f) + 2 + strings.Count(f, `"`)
		} else {
			n += len(f)
		}
	}
	if len(fields) > 0 {
		n += len(fields) - 1
	}
	return n
}

// appendCSVRecord appends the encoding of fields to dst without growing it.
// Callers must have reserved csvLen(fields) bytes of capacity if they want no
// reallocation.
func appendCSVRecord(dst []byte, fields []string) []byte {
	for i, f := range fields {
		if i > 0 {
			dst = append(dst, ',')
		}
		if !needsCSVQuote(f) {
			dst = append(dst, f...)
			continue
		}
		dst = append(dst, '"')
		for j := range len(f) {
			if f[j] == '"' {
				dst = append(dst, '"', '"')
			} else {
				dst = append(dst, f[j])
			}
		}
		dst = append(dst, '"')
	}
	return dst
}

// queryLen returns the exact encoded length of params.
func queryLen(params []Param) int {
	n := 0
	for i, p := range params {
		if i > 0 {
			n++ // '&'
		}
		n += escapedLen(p.Key) + 1 + escapedLen(p.Value) // key '=' value
	}
	return n
}

// escapedLen returns the length of s after url.QueryEscape-style escaping.
func escapedLen(s string) int {
	n := 0
	for i := range len(s) {
		if isUnreserved(s[i]) || s[i] == ' ' {
			n++
		} else {
			n += 3
		}
	}
	return n
}

const upperHex = "0123456789ABCDEF"

// appendEscaped appends s with url.QueryEscape semantics: unreserved bytes are
// kept, space becomes '+', and everything else becomes %XX with upper-case hex.
func appendEscaped(dst []byte, s string) []byte {
	for i := range len(s) {
		c := s[i]
		switch {
		case isUnreserved(c):
			dst = append(dst, c)
		case c == ' ':
			dst = append(dst, '+')
		default:
			dst = append(dst, '%', upperHex[c>>4], upperHex[c&0x0F])
		}
	}
	return dst
}

// isUnreserved reports whether c is an ASCII letter, digit, or one of "-_.~".
func isUnreserved(c byte) bool {
	switch {
	case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		return true
	}
	return c == '-' || c == '_' || c == '.' || c == '~'
}
