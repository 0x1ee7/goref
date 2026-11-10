package structs

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

var (
	// ErrInvalidEmployee reports a field that fails validation. Wrapped errors also
	// cover unparseable IDs, salaries and skill values.
	ErrInvalidEmployee = errors.New("invalid employee")
	// ErrMissingField reports a required key missing from a map record.
	ErrMissingField = errors.New("missing field")
	// ErrInvalidEmail reports an email address that fails NormalizeEmail.
	ErrInvalidEmail = errors.New("invalid email")
	// ErrCSVColumns reports a CSV row that does not have exactly seven columns.
	ErrCSVColumns = errors.New("wrong number of CSV columns")
)

// csvColumns is the number of fields in an employee CSV row.
const csvColumns = 7

// Employee is a validated employee record. All fields are unexported, so every
// Employee comes from a constructor. The zero value is an invalid, empty record.
//
// Employee is not comparable with ==, because it holds a slice. Use Equal.
type Employee struct {
	id         int
	name       string
	email      string
	department string
	salary     Money
	skills     []string
}

// NewEmployee validates the fields and returns an Employee. Name and department are
// trimmed and must be non-empty. The email is normalized. The salary must have
// a valid currency and must not be negative. Skills are normalized and
// de-duplicated.
func NewEmployee(id int, name, email, department string, salary Money, skills []string) (Employee, error) {
	if id <= 0 {
		return Employee{}, fmt.Errorf("%w: id %d must be positive", ErrInvalidEmployee, id)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Employee{}, fmt.Errorf("%w: blank name", ErrInvalidEmployee)
	}
	department = strings.TrimSpace(department)
	if department == "" {
		return Employee{}, fmt.Errorf("%w: blank department", ErrInvalidEmployee)
	}
	normEmail, err := NormalizeEmail(email)
	if err != nil {
		return Employee{}, err
	}
	if !validCurrency(salary.currency) || salary.minor < 0 {
		return Employee{}, fmt.Errorf("%w: salary must be non-negative with a currency", ErrInvalidEmployee)
	}
	return Employee{
		id:         id,
		name:       name,
		email:      normEmail,
		department: department,
		salary:     salary,
		skills:     normalizeSkills(skills),
	}, nil
}

// NewEmployeeFromMap builds an Employee from a loosely typed record, such as a
// decoded JSON object. Required keys are employee_id, name, email, department,
// salary (a decimal string) and currency. Missing keys give ErrMissingField. The
// key skills is optional. employee_id may be an int, an integral float64 or a
// decimal string. Other types give ErrInvalidEmployee. Extra keys are ignored.
func NewEmployeeFromMap(data map[string]any) (Employee, error) {
	for _, key := range []string{"employee_id", "name", "email", "department", "salary", "currency"} {
		if _, ok := data[key]; !ok {
			return Employee{}, fmt.Errorf("%w: %s", ErrMissingField, key)
		}
	}
	id, err := idFromAny(data["employee_id"])
	if err != nil {
		return Employee{}, err
	}
	name, err := stringField(data, "name")
	if err != nil {
		return Employee{}, err
	}
	email, err := stringField(data, "email")
	if err != nil {
		return Employee{}, err
	}
	dept, err := stringField(data, "department")
	if err != nil {
		return Employee{}, err
	}
	amount, err := stringField(data, "salary")
	if err != nil {
		return Employee{}, err
	}
	currency, err := stringField(data, "currency")
	if err != nil {
		return Employee{}, err
	}
	salary, err := ParseMoney(amount, currency)
	if err != nil {
		return Employee{}, fmt.Errorf("%w: salary: %w", ErrInvalidEmployee, err)
	}
	skills, err := skillsFromAny(data["skills"])
	if err != nil {
		return Employee{}, err
	}
	return NewEmployee(id, name, email, dept, salary, skills)
}

// idFromAny converts an employee_id value to an int.
func idFromAny(v any) (int, error) {
	switch v := v.(type) {
	case int:
		return v, nil
	case float64:
		if v != math.Trunc(v) || math.Abs(v) > 1<<53 {
			return 0, fmt.Errorf("%w: employee_id %v is not an integer", ErrInvalidEmployee, v)
		}
		return int(v), nil
	case string:
		id, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("%w: employee_id %q", ErrInvalidEmployee, v)
		}
		return id, nil
	default:
		return 0, fmt.Errorf("%w: employee_id has type %T", ErrInvalidEmployee, v)
	}
}

// stringField returns data[key] if it is a string. Otherwise it gives ErrInvalidEmployee.
func stringField(data map[string]any, key string) (string, error) {
	s, ok := data[key].(string)
	if !ok {
		return "", fmt.Errorf("%w: %s has type %T, want string", ErrInvalidEmployee, key, data[key])
	}
	return s, nil
}

// skillsFromAny converts the optional skills value. A missing or nil value gives nil.
func skillsFromAny(v any) ([]string, error) {
	switch v := v.(type) {
	case nil:
		return nil, nil
	case []string:
		return v, nil
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("%w: skill has type %T", ErrInvalidEmployee, item)
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%w: skills has type %T", ErrInvalidEmployee, v)
	}
}

// NewEmployeeFromCSV parses one row with the columns, in order: id, name,
// email, department, salary, currency, skills. skills is a ";"-separated list.
// A row without exactly seven columns gives ErrCSVColumns. Field errors wrap
// ErrInvalidEmployee or ErrInvalidEmail.
func NewEmployeeFromCSV(row []string) (Employee, error) {
	if len(row) != csvColumns {
		return Employee{}, fmt.Errorf("%w: got %d, want %d", ErrCSVColumns, len(row), csvColumns)
	}
	id, err := strconv.Atoi(strings.TrimSpace(row[0]))
	if err != nil {
		return Employee{}, fmt.Errorf("%w: id %q", ErrInvalidEmployee, row[0])
	}
	salary, err := ParseMoney(strings.TrimSpace(row[4]), strings.TrimSpace(row[5]))
	if err != nil {
		return Employee{}, fmt.Errorf("%w: salary: %w", ErrInvalidEmployee, err)
	}
	var skills []string
	if raw := strings.TrimSpace(row[6]); raw != "" {
		skills = strings.Split(raw, ";")
	}
	return NewEmployee(id, row[1], row[2], row[3], salary, skills)
}

// NormalizeEmail trims and lower-cases raw, then checks that it has the form
// local@domain with a non-empty local and domain and no whitespace. It returns
// ErrInvalidEmail on failure. This is a sanity check, not an RFC 5322 parser.
func NormalizeEmail(raw string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	local, domain, found := strings.Cut(s, "@")
	if !found || strings.Contains(domain, "@") || local == "" || domain == "" ||
		strings.ContainsFunc(s, unicode.IsSpace) {
		return "", fmt.Errorf("%w: %q", ErrInvalidEmail, raw)
	}
	return s, nil
}

// normalizeSkills lower-cases, trims, drops empty entries and removes duplicates,
// keeping the first occurrence. It returns nil when no skills remain.
func normalizeSkills(in []string) []string {
	var out []string
	for _, s := range in {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" || slices.Contains(out, s) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// ID returns the employee ID.
func (e Employee) ID() int { return e.id }

// Name returns the trimmed name.
func (e Employee) Name() string { return e.name }

// Email returns the normalized email address.
func (e Employee) Email() string { return e.email }

// Department returns the trimmed department name.
func (e Employee) Department() string { return e.department }

// Salary returns the salary.
func (e Employee) Salary() Money { return e.salary }

// Skills returns a copy of the skills, so callers cannot change the record. It
// returns nil when there are none.
func (e Employee) Skills() []string { return slices.Clone(e.skills) }

// Equal reports whether e and o have identical fields. Use it instead of ==, which
// does not compile for this type.
func (e Employee) Equal(o Employee) bool {
	return e.id == o.id &&
		e.name == o.name &&
		e.email == o.email &&
		e.department == o.department &&
		e.salary == o.salary &&
		slices.Equal(e.skills, o.skills)
}
