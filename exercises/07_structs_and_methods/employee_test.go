package structs_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	structs "goref/exercises/07_structs_and_methods"
)

// mustEmployee builds an Employee with NewEmployee and fails the test on error.
func mustEmployee(t *testing.T, id int, name, email, dept string, salary structs.Money, skills []string) structs.Employee {
	t.Helper()
	e, err := structs.NewEmployee(id, name, email, dept, salary, skills)
	if err != nil {
		t.Fatalf("NewEmployee(%d, %q): unexpected error: %v", id, name, err)
	}
	return e
}

// validRecord returns a map that NewEmployeeFromMap accepts.
func validRecord() map[string]any {
	return map[string]any{
		"employee_id": 7,
		"name":        "Grace Hopper",
		"email":       "Grace@Navy.example",
		"department":  "Compilers",
		"salary":      "120000.50",
		"currency":    "USD",
		"skills":      []string{"COBOL", "cobol", "Compilers"},
	}
}

func TestPart4NewEmployee(t *testing.T) {
	t.Run("normalises fields", func(t *testing.T) {
		e := mustEmployee(t, 7, "  Grace Hopper ", "  Grace@Navy.EXAMPLE ", " Compilers ",
			mustMoney(t, 12000050, "USD"), []string{" COBOL ", "cobol", "", "Compilers"})
		checks := []struct{ name, got, want string }{
			{"name", e.Name(), "Grace Hopper"},
			{"email", e.Email(), "grace@navy.example"},
			{"department", e.Department(), "Compilers"},
		}
		for _, c := range checks {
			if c.got != c.want {
				t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
			}
		}
		if e.ID() != 7 {
			t.Errorf("ID() = %d, want 7", e.ID())
		}
		if e.Salary() != mustMoney(t, 12000050, "USD") {
			t.Errorf("Salary() = %v, want 120000.50 USD", e.Salary())
		}
		if diff := cmp.Diff([]string{"cobol", "compilers"}, e.Skills()); diff != "" {
			t.Errorf("Skills mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("skills are copied on the way in and out", func(t *testing.T) {
		in := []string{"go", "sql"}
		e := mustEmployee(t, 1, "Ada", "ada@example.com", "R&D", mustMoney(t, 1, "USD"), in)
		in[0] = "rust"
		got := e.Skills()
		got[0] = "zig"
		if diff := cmp.Diff([]string{"go", "sql"}, e.Skills()); diff != "" {
			t.Errorf("Skills mutated through alias (-want +got):\n%s", diff)
		}
	})

	t.Run("no skills gives empty list", func(t *testing.T) {
		e := mustEmployee(t, 2, "Bo", "bo@example.com", "Ops", mustMoney(t, 1, "USD"), nil)
		if len(e.Skills()) != 0 {
			t.Errorf("Skills() = %v, want empty", e.Skills())
		}
	})

	t.Run("invalid fields wrap ErrInvalidEmployee", func(t *testing.T) {
		tests := []struct {
			name    string
			id      int
			ename   string
			dept    string
			salary  structs.Money
			wantErr error
		}{
			{"zero id", 0, "Ada", "R&D", mustMoney(t, 1, "USD"), structs.ErrInvalidEmployee},
			{"negative id", -3, "Ada", "R&D", mustMoney(t, 1, "USD"), structs.ErrInvalidEmployee},
			{"blank name", 1, "   ", "R&D", mustMoney(t, 1, "USD"), structs.ErrInvalidEmployee},
			{"blank department", 1, "Ada", "", mustMoney(t, 1, "USD"), structs.ErrInvalidEmployee},
			{"negative salary", 1, "Ada", "R&D", mustMoney(t, -1, "USD"), structs.ErrInvalidEmployee},
			{"zero-value salary", 1, "Ada", "R&D", structs.Money{}, structs.ErrInvalidEmployee},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_ = mustMoney(t, 1, "USD")
				_, err := structs.NewEmployee(tt.id, tt.ename, "ada@example.com", tt.dept, tt.salary, nil)
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("error = %v, want %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("invalid email wraps ErrInvalidEmail", func(t *testing.T) {
		_ = mustMoney(t, 1, "USD")
		_, err := structs.NewEmployee(1, "Ada", "not-an-email", "R&D", mustMoney(t, 1, "USD"), nil)
		if !errors.Is(err, structs.ErrInvalidEmail) {
			t.Errorf("error = %v, want ErrInvalidEmail", err)
		}
	})
}

func TestPart4NewEmployeeFromMap(t *testing.T) {
	t.Run("valid record", func(t *testing.T) {
		e, err := structs.NewEmployeeFromMap(validRecord())
		if err != nil {
			t.Fatalf("NewEmployeeFromMap: %v", err)
		}
		if e.ID() != 7 || e.Email() != "grace@navy.example" || e.Salary() != mustMoney(t, 12000050, "USD") {
			t.Errorf("got id=%d email=%q salary=%v", e.ID(), e.Email(), e.Salary())
		}
		if diff := cmp.Diff([]string{"cobol", "compilers"}, e.Skills()); diff != "" {
			t.Errorf("Skills mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("employee_id forms", func(t *testing.T) {
		tests := []struct {
			name string
			id   any
			want int
		}{
			{"int", 42, 42},
			{"integral float64 from json", float64(42), 42},
			{"decimal string", "42", 42},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				rec := validRecord()
				rec["employee_id"] = tt.id
				e, err := structs.NewEmployeeFromMap(rec)
				if err != nil {
					t.Fatalf("NewEmployeeFromMap: %v", err)
				}
				if e.ID() != tt.want {
					t.Errorf("ID() = %d, want %d", e.ID(), tt.want)
				}
			})
		}
	})

	t.Run("bad employee_id", func(t *testing.T) {
		for _, id := range []any{4.5, "forty-two", "", true, nil} {
			t.Run(fmt.Sprintf("%v", id), func(t *testing.T) {
				_ = mustMoney(t, 1, "USD")
				rec := validRecord()
				rec["employee_id"] = id
				if _, err := structs.NewEmployeeFromMap(rec); !errors.Is(err, structs.ErrInvalidEmployee) {
					t.Errorf("error = %v, want ErrInvalidEmployee", err)
				}
			})
		}
	})

	t.Run("missing required key", func(t *testing.T) {
		for _, key := range []string{"employee_id", "name", "email", "department", "salary", "currency"} {
			t.Run(key, func(t *testing.T) {
				_ = mustMoney(t, 1, "USD")
				rec := validRecord()
				delete(rec, key)
				if _, err := structs.NewEmployeeFromMap(rec); !errors.Is(err, structs.ErrMissingField) {
					t.Errorf("error = %v, want ErrMissingField", err)
				}
			})
		}
	})

	t.Run("salary must be a string", func(t *testing.T) {
		rec := validRecord()
		rec["salary"] = 120000.5
		_ = mustMoney(t, 1, "USD")
		if _, err := structs.NewEmployeeFromMap(rec); !errors.Is(err, structs.ErrInvalidEmployee) {
			t.Errorf("float salary error = %v, want ErrInvalidEmployee", err)
		}
	})

	t.Run("salary string is parsed exactly", func(t *testing.T) {
		rec := validRecord()
		rec["salary"] = "0.1"
		rec["currency"] = "USD"
		_ = mustMoney(t, 1, "USD")
		e, err := structs.NewEmployeeFromMap(rec)
		if err != nil {
			t.Fatalf("NewEmployeeFromMap: %v", err)
		}
		if e.Salary() != mustMoney(t, 10, "USD") {
			t.Errorf("Salary() = %v, want 0.10 USD", e.Salary())
		}
	})

	t.Run("skills forms", func(t *testing.T) {
		tests := []struct {
			name   string
			skills any
			want   []string
		}{
			{"[]string", []string{"Go"}, []string{"go"}},
			{"[]any of strings", []any{"Go", "SQL"}, []string{"go", "sql"}},
			{"absent", nil, nil},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				rec := validRecord()
				if tt.skills == nil {
					delete(rec, "skills")
				} else {
					rec["skills"] = tt.skills
				}
				e, err := structs.NewEmployeeFromMap(rec)
				if err != nil {
					t.Fatalf("NewEmployeeFromMap: %v", err)
				}
				if diff := cmp.Diff(tt.want, e.Skills()); diff != "" {
					t.Errorf("Skills mismatch (-want +got):\n%s", diff)
				}
			})
		}
	})

	t.Run("non-string skill", func(t *testing.T) {
		_ = mustMoney(t, 1, "USD")
		rec := validRecord()
		rec["skills"] = []any{"go", 3}
		if _, err := structs.NewEmployeeFromMap(rec); !errors.Is(err, structs.ErrInvalidEmployee) {
			t.Errorf("error = %v, want ErrInvalidEmployee", err)
		}
	})

	t.Run("extra keys are ignored", func(t *testing.T) {
		rec := validRecord()
		rec["nickname"] = "Amazing Grace"
		if _, err := structs.NewEmployeeFromMap(rec); err != nil {
			t.Errorf("NewEmployeeFromMap with extra key: %v", err)
		}
	})
}

func TestPart4NewEmployeeFromCSV(t *testing.T) {
	t.Run("valid row", func(t *testing.T) {
		row := []string{"12", " Alan Turing ", "Alan@Bletchley.example", "Research", "99999.99", "GBP", "Logic; Cryptanalysis ;logic"}
		e, err := structs.NewEmployeeFromCSV(row)
		if err != nil {
			t.Fatalf("NewEmployeeFromCSV: %v", err)
		}
		if e.ID() != 12 || e.Name() != "Alan Turing" || e.Email() != "alan@bletchley.example" {
			t.Errorf("got id=%d name=%q email=%q", e.ID(), e.Name(), e.Email())
		}
		if e.Salary() != mustMoney(t, 9999999, "GBP") {
			t.Errorf("Salary() = %v, want 99999.99 GBP", e.Salary())
		}
		if diff := cmp.Diff([]string{"logic", "cryptanalysis"}, e.Skills()); diff != "" {
			t.Errorf("Skills mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("empty skills column", func(t *testing.T) {
		row := []string{"3", "Ada", "ada@example.com", "R&D", "1.00", "USD", ""}
		e, err := structs.NewEmployeeFromCSV(row)
		if err != nil {
			t.Fatalf("NewEmployeeFromCSV: %v", err)
		}
		if len(e.Skills()) != 0 {
			t.Errorf("Skills() = %v, want empty", e.Skills())
		}
	})

	t.Run("wrong column count", func(t *testing.T) {
		good := []string{"3", "Ada", "ada@example.com", "R&D", "1.00", "USD", "go"}
		tests := map[string][]string{
			"six columns":   good[:6],
			"eight columns": append(append([]string{}, good...), "extra"),
			"no columns":    nil,
		}
		for name, row := range tests {
			t.Run(name, func(t *testing.T) {
				_ = mustMoney(t, 1, "USD")
				if _, err := structs.NewEmployeeFromCSV(row); !errors.Is(err, structs.ErrCSVColumns) {
					t.Errorf("error = %v, want ErrCSVColumns", err)
				}
			})
		}
	})

	t.Run("bad fields", func(t *testing.T) {
		tests := []struct {
			name    string
			col     int
			val     string
			wantErr error
		}{
			{"non-numeric id", 0, "x12", structs.ErrInvalidEmployee},
			{"bad salary", 4, "lots", structs.ErrInvalidEmployee},
			{"bad email", 2, "ada at example", structs.ErrInvalidEmail},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_ = mustMoney(t, 1, "USD")
				valid := []string{"3", "Ada", "ada@example.com", "R&D", "1.00", "USD", "go"}
				if _, err := structs.NewEmployeeFromCSV(valid); err != nil {
					t.Fatalf("valid row rejected: %v", err)
				}
				row := append([]string{}, valid...)
				row[tt.col] = tt.val
				if _, err := structs.NewEmployeeFromCSV(row); !errors.Is(err, tt.wantErr) {
					t.Errorf("error = %v, want %v", err, tt.wantErr)
				}
			})
		}
	})
}

func TestPart4NormalizeEmail(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		tests := map[string]string{
			"  Ada@Example.COM  ": "ada@example.com",
			"first.last+tag@x.io": "first.last+tag@x.io",
		}
		for in, want := range tests {
			t.Run(in, func(t *testing.T) {
				_ = mustMoney(t, 1, "USD")
				got, err := structs.NormalizeEmail(in)
				if err != nil {
					t.Fatalf("NormalizeEmail(%q): %v", in, err)
				}
				if got != want {
					t.Errorf("NormalizeEmail(%q) = %q, want %q", in, got, want)
				}
			})
		}
	})

	t.Run("invalid", func(t *testing.T) {
		for _, in := range []string{"", "ada", "a@b@c", "@example.com", "ada@", "a b@x.com", "a@b c.com", "a@@b"} {
			t.Run(fmt.Sprintf("%q", in), func(t *testing.T) {
				_ = mustMoney(t, 1, "USD")
				if _, err := structs.NormalizeEmail(in); !errors.Is(err, structs.ErrInvalidEmail) {
					t.Errorf("NormalizeEmail(%q) error = %v, want ErrInvalidEmail", in, err)
				}
			})
		}
	})
}

func TestPart4EmployeeValue(t *testing.T) {
	t.Run("equal compares every field", func(t *testing.T) {
		base := mustEmployee(t, 1, "Ada", "ada@example.com", "R&D", mustMoney(t, 100, "USD"), []string{"go"})
		same := mustEmployee(t, 1, "Ada", "ada@example.com", "R&D", mustMoney(t, 100, "USD"), []string{"go"})
		if !base.Equal(same) {
			t.Errorf("identical employees not Equal")
		}
		other := mustEmployee(t, 1, "Ada", "ada@example.com", "R&D", mustMoney(t, 100, "USD"), []string{"rust"})
		if base.Equal(other) {
			t.Errorf("employees with different skills are Equal")
		}
	})

	t.Run("zero value is not a valid employee", func(t *testing.T) {
		_ = mustEmployee(t, 1, "Ada", "ada@example.com", "R&D", mustMoney(t, 1, "USD"), nil)
		if _, err := structs.NewEmployee(0, "", "", "", structs.Money{}, nil); err == nil {
			t.Fatalf("NewEmployee with empty fields accepted")
		}
		var zero structs.Employee
		if zero.ID() != 0 || zero.Name() != "" || zero.Skills() != nil {
			t.Errorf("zero Employee: id=%d name=%q skills=%v", zero.ID(), zero.Name(), zero.Skills())
		}
		if !zero.Equal(structs.Employee{}) {
			t.Errorf("two zero Employees are not Equal")
		}
	})
}
