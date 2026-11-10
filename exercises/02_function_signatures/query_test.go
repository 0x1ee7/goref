package signatures_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"goref/exercises/02_function_signatures"
)

type buildCase struct {
	name     string
	query    signatures.Query
	wantSQL  string
	wantArgs []any
	wantErr  error
}

func runBuildCases(t *testing.T, cases []buildCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sql, args, err := tc.query.Build()
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Build() error = %v, want errors.Is %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Build() unexpected error: %v", err)
			}
			if sql != tc.wantSQL {
				t.Errorf("Build() sql =\n  %q\nwant\n  %q", sql, tc.wantSQL)
			}
			if diff := cmp.Diff(tc.wantArgs, args, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("Build() args mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func mustBuild(t *testing.T, q signatures.Query) (string, []any) {
	t.Helper()
	sql, args, err := q.Build()
	if err != nil {
		t.Fatalf("Build() unexpected error: %v", err)
	}
	return sql, args
}

func TestPart1Select(t *testing.T) {
	spread := []string{"id", "name"}
	runBuildCases(t, []buildCase{
		{
			name:    "no columns selects star",
			query:   signatures.Select("users"),
			wantSQL: "SELECT * FROM users",
		},
		{
			name:    "variadic columns",
			query:   signatures.Select("users", "id", "email"),
			wantSQL: "SELECT id, email FROM users",
		},
		{
			name:    "spread slice",
			query:   signatures.Select("users", spread...),
			wantSQL: "SELECT id, name FROM users",
		},
		{
			name:    "nil spread is same as no columns",
			query:   signatures.Select("users", []string(nil)...),
			wantSQL: "SELECT * FROM users",
		},
		{
			name:    "dotted table and column",
			query:   signatures.Select("public.users", "u.id"),
			wantSQL: "SELECT u.id FROM public.users",
		},
		{
			name:    "underscore identifiers",
			query:   signatures.Select("_audit", "_row_id"),
			wantSQL: "SELECT _row_id FROM _audit",
		},
		{
			name:    "invalid table with injection text",
			query:   signatures.Select("users; DROP TABLE users"),
			wantErr: signatures.ErrInvalidIdentifier,
		},
		{
			name:    "expression column is not an identifier",
			query:   signatures.Select("users", "count(*)"),
			wantErr: signatures.ErrInvalidIdentifier,
		},
		{
			name:    "leading digit is invalid",
			query:   signatures.Select("1users"),
			wantErr: signatures.ErrInvalidIdentifier,
		},
		{
			name:    "empty table is invalid",
			query:   signatures.Select(""),
			wantErr: signatures.ErrInvalidIdentifier,
		},
		{
			name:    "two dots is invalid",
			query:   signatures.Select("a.b.c"),
			wantErr: signatures.ErrInvalidIdentifier,
		},
	})
}

func TestPart1ColumnsAreCopied(t *testing.T) {
	cols := []string{"id", "email"}
	q := signatures.Select("users", cols...)

	cols[0] = "password"
	cols = append(cols, "secret")
	_ = cols

	sql, _ := mustBuild(t, q)
	if want := "SELECT id, email FROM users"; sql != want {
		t.Errorf("after mutating the caller's slice, sql = %q, want %q", sql, want)
	}
}

func TestPart2ChainedOptions(t *testing.T) {
	runBuildCases(t, []buildCase{
		{
			name: "README example",
			query: signatures.Select("users", "id", "email").
				Where(signatures.Eq("active", true), signatures.Eq("deleted_at", nil)).
				OrderBy("-id").
				Limit(20),
			wantSQL:  "SELECT id, email FROM users WHERE active = ? AND deleted_at IS NULL ORDER BY id DESC LIMIT ?",
			wantArgs: []any{true, 20},
		},
		{
			name:     "where calls accumulate with AND",
			query:    signatures.Select("users").Where(signatures.Eq("a", 1)).Where(signatures.Eq("b", 2)),
			wantSQL:  "SELECT * FROM users WHERE a = ? AND b = ?",
			wantArgs: []any{1, 2},
		},
		{
			name:    "empty where adds no clause",
			query:   signatures.Select("users").Where(),
			wantSQL: "SELECT * FROM users",
		},
		{
			name:    "nil value renders IS NULL without parameter",
			query:   signatures.Select("users").Where(signatures.Eq("x", nil)),
			wantSQL: "SELECT * FROM users WHERE x IS NULL",
		},
		{
			name:    "dotted where column",
			query:   signatures.Select("users", "u.id").Where(signatures.Eq("u.deleted_at", nil)),
			wantSQL: "SELECT u.id FROM users WHERE u.deleted_at IS NULL",
		},
		{
			name:    "order terms in one call and across calls",
			query:   signatures.Select("events").OrderBy("name", "-created_at").OrderBy("id"),
			wantSQL: "SELECT * FROM events ORDER BY name, created_at DESC, id",
		},
		{
			name:     "limit zero is a valid bound parameter",
			query:    signatures.Select("users").Limit(0),
			wantSQL:  "SELECT * FROM users LIMIT ?",
			wantArgs: []any{0},
		},
		{
			name:     "last limit wins",
			query:    signatures.Select("users").Limit(5).Limit(10),
			wantSQL:  "SELECT * FROM users LIMIT ?",
			wantArgs: []any{10},
		},
		{
			name:    "negative limit is ErrInvalidLimit",
			query:   signatures.Select("users").Limit(-1),
			wantErr: signatures.ErrInvalidLimit,
		},
		{
			name:    "bad where column",
			query:   signatures.Select("users").Where(signatures.Eq("a b", 1)),
			wantErr: signatures.ErrInvalidIdentifier,
		},
		{
			name:    "bare DESC marker is invalid",
			query:   signatures.Select("users").OrderBy("-"),
			wantErr: signatures.ErrInvalidIdentifier,
		},
		{
			name:    "order term with punctuation is invalid",
			query:   signatures.Select("users").OrderBy("name;"),
			wantErr: signatures.ErrInvalidIdentifier,
		},
		{
			name:    "error from Limit survives later valid calls",
			query:   signatures.Select("users").Limit(-1).Where(signatures.Eq("a", 1)).OrderBy("id").Limit(5),
			wantErr: signatures.ErrInvalidLimit,
		},
		{
			name:    "error from Select survives chaining",
			query:   signatures.Select("bad table").Where(signatures.Eq("a", 1)).Limit(5),
			wantErr: signatures.ErrInvalidIdentifier,
		},
	})
}

func TestPart2BuilderIsImmutable(t *testing.T) {
	base := signatures.Select("users").Where(signatures.Eq("a", 1))

	branch := base.Where(signatures.Eq("b", 2)).OrderBy("id").Limit(3)
	other := base.OrderBy("-a")

	if sql, args := mustBuild(t, base); sql != "SELECT * FROM users WHERE a = ?" || !cmp.Equal(args, []any{1}) {
		t.Errorf("base changed after deriving branches: sql=%q args=%v", sql, args)
	}
	if sql, args := mustBuild(t, branch); sql != "SELECT * FROM users WHERE a = ? AND b = ? ORDER BY id LIMIT ?" || !cmp.Equal(args, []any{1, 2, 3}) {
		t.Errorf("branch: sql=%q args=%v", sql, args)
	}
	if sql, args := mustBuild(t, other); sql != "SELECT * FROM users WHERE a = ? ORDER BY a DESC" || !cmp.Equal(args, []any{1}) {
		t.Errorf("other branch: sql=%q args=%v", sql, args)
	}
}

func TestPart2BuilderReuseAfterBranching(t *testing.T) {
	// Appending to a shared base must not let two branches overwrite each other's
	// conditions through a shared backing array.
	base := signatures.Select("t").Where(signatures.Eq("a", 1))
	left := base.Where(signatures.Eq("left", 2))
	right := base.Where(signatures.Eq("right", 3))

	if sql, args := mustBuild(t, left); sql != "SELECT * FROM t WHERE a = ? AND left = ?" || !cmp.Equal(args, []any{1, 2}) {
		t.Errorf("left branch overwritten: sql=%q args=%v", sql, args)
	}
	if sql, args := mustBuild(t, right); sql != "SELECT * FROM t WHERE a = ? AND right = ?" || !cmp.Equal(args, []any{1, 3}) {
		t.Errorf("right branch: sql=%q args=%v", sql, args)
	}
}
