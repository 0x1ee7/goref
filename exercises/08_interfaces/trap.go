package interfaces

// Severity classifies a validation finding (Part 6).
type Severity int

// Severities, least to most serious.
const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityError
)

// String implements fmt.Stringer: "info", "warning", "error", and
// "Severity(N)" for anything else.
func (s Severity) String() string { return "" }

// FieldError is one failed check on one field.
type FieldError struct {
	Severity Severity
	Field    string
	Reason   string
}

// Error implements error as "Field: Reason".
func (e FieldError) Error() string { return "" }

// ErrorList accumulates FieldErrors. Its methods are nil-safe where noted.
type ErrorList struct{}

// Add appends a finding. Not nil-safe.
func (l *ErrorList) Add(sev Severity, field, reason string) {}

// Len returns the number of findings; nil-safe.
func (l *ErrorList) Len() int { return 0 }

// Errors returns a copy of the findings; nil-safe.
func (l *ErrorList) Errors() []FieldError { return nil }

// Error implements error; nil-safe.
func (l *ErrorList) Error() string { return "" }

// User is the input to ValidateUser.
type User struct {
	Name  string
	Age   int
	Email string
}

// ValidateUser checks u and returns nil when it is valid. Otherwise it returns a
// *ErrorList with every failure in field order. The function must return an
// untyped nil on success (Part 6).
func ValidateUser(u User) error { return nil }

// IsNilValue reports whether v is an untyped nil or a nil pointer, map, slice,
// func, chan or interface wrapped in an interface value (Part 6).
func IsNilValue(v any) bool { return false }
