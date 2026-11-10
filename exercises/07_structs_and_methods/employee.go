package structs

import "errors"

var (
	// ErrInvalidEmployee reports a field of an employee record that fails validation.
	ErrInvalidEmployee = errors.New("invalid employee")
	// ErrMissingField reports a required key absent from a map-based record.
	ErrMissingField = errors.New("missing field")
	// ErrInvalidEmail reports an email address that is not a plain local@domain.
	ErrInvalidEmail = errors.New("invalid email")
	// ErrCSVColumns reports a CSV row that does not have exactly seven columns.
	ErrCSVColumns = errors.New("wrong number of csv columns")
)

// Employee is a validated employee record. Fields are unexported so every value
// comes through a constructor and satisfies the invariants. Use NewEmployee,
// NewEmployeeFromMap or NewEmployeeFromCSV. The zero value is not valid.
//
// Employee holds a slice, so it is not comparable with ==. Use Equal.
type Employee struct {
	id         int
	name       string
	email      string
	department string
	salary     Money
	skills     []string
}

// NewEmployee validates the fields and returns an Employee. Name and department
// are trimmed and must be non-empty. The email is normalised with NormalizeEmail.
// The salary must have a valid currency and be >= 0. Skills are trimmed,
// lowercased, de-duplicated (first occurrence kept) and empty ones dropped. The
// input slice is copied.
func NewEmployee(id int, name, email, department string, salary Money, skills []string) (Employee, error) {
	return Employee{}, errNotImplemented
}

// NewEmployeeFromMap builds an Employee from a loosely typed record such as a
// decoded JSON object. Required keys: employee_id, name, email, department,
// salary (a string), currency. Optional: skills ([]string or []any of strings).
// Extra keys are ignored. Missing keys are ErrMissingField. employee_id may be an
// int, an integral float64 (as produced by encoding/json), or a decimal string.
func NewEmployeeFromMap(data map[string]any) (Employee, error) {
	return Employee{}, errNotImplemented
}

// NewEmployeeFromCSV builds an Employee from a seven-column row:
// employee_id, name, email, department, salary, currency, skills.
// skills is a ';'-separated list and may be empty. Any other column count is
// ErrCSVColumns.
func NewEmployeeFromCSV(row []string) (Employee, error) {
	return Employee{}, errNotImplemented
}

// NormalizeEmail trims and lowercases raw, then checks that it has exactly one
// '@', a non-empty local part and domain, and no whitespace. Otherwise it is
// ErrInvalidEmail.
func NormalizeEmail(raw string) (string, error) { return "", errNotImplemented }

// ID returns the employee number (> 0).
func (e Employee) ID() int { return 0 }

// Name returns the trimmed name.
func (e Employee) Name() string { return "" }

// Email returns the normalised email address.
func (e Employee) Email() string { return "" }

// Department returns the trimmed department.
func (e Employee) Department() string { return "" }

// Salary returns the annual salary.
func (e Employee) Salary() Money { return Money{} }

// Skills returns a copy of the normalised skill list. Mutating it does not affect e.
func (e Employee) Skills() []string { return nil }

// Equal reports whether e and o hold the same values field by field.
func (e Employee) Equal(o Employee) bool { return false }
