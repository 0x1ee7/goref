package perf

// Param is one key/value pair of a URL query string.
type Param struct {
	Key   string
	Value string
}

// CSVRecord encodes fields as one RFC 4180 record, without a trailing newline.
// A field is quoted only if it contains a comma, double quote, CR or LF; embedded
// quotes are doubled. CSVRecord performs exactly one allocation for any non-empty
// input (the returned string).
func CSVRecord(fields []string) string {
	return ""
}

// AppendCSVRecord appends the CSVRecord encoding of fields to dst and returns the
// extended slice. It grows dst at most once, and not at all when dst already has
// enough spare capacity.
func AppendCSVRecord(dst []byte, fields []string) []byte {
	return dst
}

// QueryString encodes params as an application/x-www-form-urlencoded query
// string, in the order given, joined with '&'. Keys and values use the escaping
// of url.QueryEscape. QueryString performs exactly one allocation for any
// non-empty input.
func QueryString(params []Param) string {
	return ""
}
