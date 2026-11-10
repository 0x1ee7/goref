package interfaces

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

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
func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	default:
		return fmt.Sprintf("Severity(%d)", int(s))
	}
}

// FieldError is one failed check on one field.
type FieldError struct {
	Severity Severity
	Field    string
	Reason   string
}

// Error implements error as "Field: Reason".
func (e FieldError) Error() string { return e.Field + ": " + e.Reason }

// ErrorList accumulates FieldErrors. Its methods are nil-safe where noted.
type ErrorList struct {
	items []FieldError
}

// Add appends a finding. Not nil-safe.
func (l *ErrorList) Add(sev Severity, field, reason string) {
	l.items = append(l.items, FieldError{Severity: sev, Field: field, Reason: reason})
}

// Len returns the number of findings; nil-safe.
func (l *ErrorList) Len() int {
	if l == nil {
		return 0
	}
	return len(l.items)
}

// Errors returns a copy of the findings; nil-safe.
func (l *ErrorList) Errors() []FieldError {
	if l == nil {
		return nil
	}
	return slices.Clone(l.items)
}

// Error implements error by joining the findings with "; "; nil-safe.
func (l *ErrorList) Error() string {
	if l == nil {
		return ""
	}
	parts := make([]string, len(l.items))
	for i, fe := range l.items {
		parts[i] = fe.Error()
	}
	return strings.Join(parts, "; ")
}

// User is the input to ValidateUser.
type User struct {
	Name  string
	Age   int
	Email string
}

// ValidateUser checks u and returns nil when it is valid. Otherwise it returns a
// *ErrorList with every failure in field order. The function returns an untyped
// nil on success, never a typed nil *ErrorList (Part 6).
func ValidateUser(u User) error {
	var list ErrorList
	if u.Name == "" {
		list.Add(SeverityError, "Name", "is required")
	}
	if u.Age < 0 || u.Age > 150 {
		list.Add(SeverityError, "Age", "must be between 0 and 150")
	}
	switch {
	case u.Email == "":
		list.Add(SeverityError, "Email", "is required")
	case !strings.Contains(u.Email, "@"):
		list.Add(SeverityError, "Email", "must contain '@'")
	}
	if list.Len() == 0 {
		return nil
	}
	return &list
}

// IsNilValue reports whether v is an untyped nil or a nil pointer, map, slice,
// func, chan or interface wrapped in an interface value (Part 6).
func IsNilValue(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice, reflect.UnsafePointer:
		return rv.IsNil()
	default:
		return false
	}
}
