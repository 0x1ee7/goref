// Package signatures demonstrates Go function and method signature design:
// functional options, optional/PATCH semantics, small consumer-side interfaces,
// error values, immutable builders, and generics.
package signatures

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// ErrInvalidIdentifier reports a table, column, or order term that is not a
// plain SQL identifier. The builder never quotes or escapes identifiers, so
// anything outside the allowed shape is rejected instead.
var ErrInvalidIdentifier = errors.New("invalid SQL identifier")

// ErrInvalidLimit reports a negative LIMIT.
var ErrInvalidLimit = errors.New("invalid LIMIT")

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`)

func checkIdent(name string) error {
	if !identifier.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrInvalidIdentifier, name)
	}
	return nil
}

// Cond is a single equality condition. A nil value renders as IS NULL and
// binds no parameter.
type Cond struct {
	col string
	val any
}

// Eq returns the condition col = v, or col IS NULL when v is nil. Validation of
// col is deferred to Query.Where so that Eq stays a pure value constructor.
func Eq(col string, v any) Cond {
	return Cond{col: col, val: v}
}

// Query is an immutable SELECT builder. Every method returns a new Query and
// never modifies its receiver, so a partially built query can be shared and
// branched safely. The first invalid input is remembered and reported by Build.
type Query struct {
	table    string
	cols     []string
	conds    []Cond
	orders   []string
	limit    int
	hasLimit bool
	err      error
}

// Select starts a query on table. With no columns it selects *. The column
// slice is copied, so later mutation by the caller has no effect.
func Select(table string, cols ...string) Query {
	q := Query{table: table}
	if err := checkIdent(table); err != nil {
		q.err = err
		return q
	}
	for _, c := range cols {
		if err := checkIdent(c); err != nil {
			q.err = err
			return q
		}
	}
	q.cols = slices.Clone(cols)
	return q
}

func (q Query) fail(err error) Query {
	if q.err == nil {
		q.err = err
	}
	return q
}

// Where adds conditions joined with AND. Calling Where with no conditions is a no-op.
func (q Query) Where(conds ...Cond) Query {
	for _, c := range conds {
		if err := checkIdent(c.col); err != nil {
			return q.fail(err)
		}
	}
	// Clip forces append to allocate, so branches never share a backing array.
	q.conds = append(slices.Clip(q.conds), conds...)
	return q
}

// OrderBy adds ORDER BY terms. A leading "-" means DESC.
func (q Query) OrderBy(terms ...string) Query {
	rendered := make([]string, 0, len(terms))
	for _, term := range terms {
		name, desc := strings.CutPrefix(term, "-")
		if err := checkIdent(name); err != nil {
			return q.fail(err)
		}
		if desc {
			rendered = append(rendered, name+" DESC")
		} else {
			rendered = append(rendered, name)
		}
	}
	q.orders = append(slices.Clip(q.orders), rendered...)
	return q
}

// Limit sets the row limit. The last call wins. Negative values are an error.
func (q Query) Limit(n int) Query {
	if n < 0 {
		return q.fail(fmt.Errorf("%w: %d", ErrInvalidLimit, n))
	}
	q.limit, q.hasLimit = n, true
	return q
}

// Build renders the SQL text and its bound arguments. Values are always passed
// as parameters, never interpolated.
func (q Query) Build() (string, []any, error) {
	if q.err != nil {
		return "", nil, q.err
	}
	var b strings.Builder
	var args []any

	b.WriteString("SELECT ")
	if len(q.cols) == 0 {
		b.WriteString("*")
	} else {
		b.WriteString(strings.Join(q.cols, ", "))
	}
	b.WriteString(" FROM ")
	b.WriteString(q.table)

	for i, c := range q.conds {
		if i == 0 {
			b.WriteString(" WHERE ")
		} else {
			b.WriteString(" AND ")
		}
		if c.val == nil {
			b.WriteString(c.col + " IS NULL")
			continue
		}
		b.WriteString(c.col + " = ?")
		args = append(args, c.val)
	}

	if len(q.orders) > 0 {
		b.WriteString(" ORDER BY ")
		b.WriteString(strings.Join(q.orders, ", "))
	}
	if q.hasLimit {
		b.WriteString(" LIMIT ?")
		args = append(args, q.limit)
	}
	return b.String(), args, nil
}
