// Package signatures covers Go's calling conventions through six small
// services: a query builder, an HTTP client, PATCH semantics, dependency
// wiring, an event bus and deterministic cache keys.
package signatures

import "errors"

// ErrInvalidIdentifier is wrapped by Build when a table, column, or ORDER BY
// term is not a (optionally dotted) SQL identifier.
var ErrInvalidIdentifier = errors.New("invalid identifier")

// ErrInvalidLimit is wrapped by Build when Limit was given a negative value.
var ErrInvalidLimit = errors.New("invalid limit")

// Cond is a WHERE condition.
type Cond struct{}

// Eq returns the condition col = ? with v bound as a parameter. A nil v
// renders as col IS NULL with no parameter.
func Eq(col string, v any) Cond { return Cond{} }

// Query is an immutable SELECT builder. Every method returns a new Query and
// never modifies its receiver. Errors are sticky: the first invalid input is
// remembered and reported by Build.
type Query struct{}

// Select starts a query over table. cols is variadic: no columns renders "*".
// The function must copy cols, so later changes to the caller's slice do not
// affect the query.
func Select(table string, cols ...string) Query { return Query{} }

// Where appends conditions joined with AND. Calling Where with no conditions
// adds no clause.
func (q Query) Where(conds ...Cond) Query { return q }

// OrderBy appends ORDER BY terms. A leading "-" means DESC.
func (q Query) OrderBy(terms ...string) Query { return q }

// Limit sets the LIMIT. The last call wins. A negative n is an error.
func (q Query) Limit(n int) Query { return q }

// Build renders the SQL with "?" placeholders and the matching arguments in
// placeholder order.
func (q Query) Build() (sql string, args []any, err error) { return "", nil, nil }
