package signatures_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"goref/exercises/02_function_signatures"
)

func baseUser() signatures.User {
	return signatures.User{ID: 7, Email: "ana@example.com", Nickname: new("ana")}
}

func decodePatch(t *testing.T, body string) signatures.UserPatch {
	t.Helper()
	var p signatures.UserPatch
	if err := json.Unmarshal([]byte(body), &p); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return p
}

func TestPart3ApplyPatchFromJSON(t *testing.T) {
	ana := baseUser()
	tests := []struct {
		name    string
		body    string
		want    signatures.User
		wantErr error
	}{
		{name: "empty object leaves user alone", body: `{}`, want: ana},
		{
			name: "absent nickname is untouched and email is normalized",
			body: `{"email":"  Ana.New@Example.COM "}`,
			want: signatures.User{ID: 7, Email: "ana.new@example.com", Nickname: new("ana")},
		},
		{
			name: "explicit null clears nickname",
			body: `{"nickname":null}`,
			want: signatures.User{ID: 7, Email: "ana@example.com", Nickname: nil},
		},
		{
			name: "value nickname is trimmed",
			body: `{"nickname":"  bo  "}`,
			want: signatures.User{ID: 7, Email: "ana@example.com", Nickname: new("bo")},
		},
		{
			name: "32 characters is the limit",
			body: fmt.Sprintf(`{"nickname":%q}`, strings.Repeat("x", 32)),
			want: signatures.User{ID: 7, Email: "ana@example.com", Nickname: new(strings.Repeat("x", 32))},
		},
		{
			name: "length counts characters, not bytes",
			body: fmt.Sprintf(`{"nickname":%q}`, strings.Repeat("é", 32)),
			want: signatures.User{ID: 7, Email: "ana@example.com", Nickname: new(strings.Repeat("é", 32))},
		},
		{name: "null email cannot clear", body: `{"email":null}`, wantErr: signatures.ErrInvalidEmail},
		{name: "email without dot after @", body: `{"email":"ana@localhost"}`, wantErr: signatures.ErrInvalidEmail},
		{name: "email with inner space", body: `{"email":"ana smith@example.com"}`, wantErr: signatures.ErrInvalidEmail},
		{name: "blank nickname", body: `{"nickname":"   "}`, wantErr: signatures.ErrInvalidNickname},
		{name: "33 characters is too long", body: fmt.Sprintf(`{"nickname":%q}`, strings.Repeat("x", 33)), wantErr: signatures.ErrInvalidNickname},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := signatures.ApplyPatch(baseUser(), decodePatch(t, tc.body))
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("ApplyPatch error = %v, want errors.Is %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ApplyPatch unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ApplyPatch mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPart3ApplyPatchDoesNotMutateInput(t *testing.T) {
	t.Run("value nickname", func(t *testing.T) {
		base := baseUser()
		got, err := signatures.ApplyPatch(base, decodePatch(t, `{"nickname":"zed"}`))
		if err != nil {
			t.Fatalf("ApplyPatch unexpected error: %v", err)
		}
		if got.Nickname == nil || *got.Nickname != "zed" {
			t.Fatalf("patched nickname = %v, want zed", got.Nickname)
		}
		if base.Nickname == nil || *base.Nickname != "ana" {
			t.Errorf("input user's nickname changed through the pointer: %v", base.Nickname)
		}
	})
	t.Run("clearing nickname", func(t *testing.T) {
		base := baseUser()
		got, err := signatures.ApplyPatch(base, decodePatch(t, `{"nickname":null}`))
		if err != nil {
			t.Fatalf("ApplyPatch unexpected error: %v", err)
		}
		if got.ID != base.ID || got.Email != base.Email {
			t.Fatalf("patched user = %+v, want ID and Email of %+v", got, base)
		}
		if got.Nickname != nil {
			t.Fatalf("cleared nickname = %q, want nil", *got.Nickname)
		}
		if base.Nickname == nil || *base.Nickname != "ana" {
			t.Errorf("input user lost its nickname: %v", base.Nickname)
		}
	})
}

func TestPart3Optional(t *testing.T) {
	t.Run("some value", func(t *testing.T) {
		o := signatures.Some("x")
		if !o.IsSet() || o.IsNull() || o.IsZero() {
			t.Errorf("Some(\"x\"): IsSet=%v IsNull=%v IsZero=%v", o.IsSet(), o.IsNull(), o.IsZero())
		}
		if v, ok := o.Get(); v != "x" || !ok {
			t.Errorf("Get() = (%q, %v), want (\"x\", true)", v, ok)
		}
	})
	t.Run("explicit null", func(t *testing.T) {
		o := signatures.Null[string]()
		if !o.IsSet() || !o.IsNull() {
			t.Errorf("Null(): IsSet=%v IsNull=%v, want both true", o.IsSet(), o.IsNull())
		}
		if v, ok := o.Get(); v != "" || ok {
			t.Errorf("Get() = (%q, %v), want (\"\", false)", v, ok)
		}
	})
	t.Run("zero value is unset", func(t *testing.T) {
		var o signatures.Optional[int]
		if o.IsSet() || o.IsNull() {
			t.Errorf("zero Optional: IsSet=%v IsNull=%v, want both false", o.IsSet(), o.IsNull())
		}
		if !o.IsZero() {
			t.Errorf("zero Optional: IsZero() = false, want true")
		}
	})
	t.Run("some zero value is still set", func(t *testing.T) {
		o := signatures.Some(0)
		if !o.IsSet() {
			t.Errorf("Some(0).IsSet() = false, want true")
		}
		if o.IsZero() {
			t.Errorf("Some(0).IsZero() = true, want false (omitzero would drop it)")
		}
		if v, ok := o.Get(); v != 0 || !ok {
			t.Errorf("Get() = (%d, %v), want (0, true)", v, ok)
		}
	})
}

func TestPart3DecodeNullIntoLocalStruct(t *testing.T) {
	type counter struct {
		N signatures.Optional[int] `json:"n,omitzero"`
	}
	t.Run("explicit null", func(t *testing.T) {
		var c counter
		if err := json.Unmarshal([]byte(`{"n":null}`), &c); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !c.N.IsSet() || !c.N.IsNull() {
			t.Errorf("{\"n\":null}: IsSet=%v IsNull=%v, want both true", c.N.IsSet(), c.N.IsNull())
		}
	})
	t.Run("absent", func(t *testing.T) {
		var c counter
		if err := json.Unmarshal([]byte(`{}`), &c); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if c.N.IsSet() {
			t.Errorf("{}: IsSet() = true, want false")
		}
		if out, _ := json.Marshal(c); string(out) != `{}` {
			t.Errorf("absent field marshals as %s, want {}", out)
		}
	})
}

func TestPart3MarshalPatch(t *testing.T) {
	type counter struct {
		N signatures.Optional[int] `json:"n,omitzero"`
	}
	tests := []struct {
		name string
		in   any
		want string
	}{
		{name: "empty patch omits everything", in: signatures.UserPatch{}, want: `{}`},
		{name: "null is emitted", in: signatures.UserPatch{Nickname: signatures.Null[string]()}, want: `{"nickname":null}`},
		{
			name: "empty string value is emitted",
			in:   signatures.UserPatch{Email: signatures.Some("a@b.co"), Nickname: signatures.Some("")},
			want: `{"email":"a@b.co","nickname":""}`,
		},
		{name: "zero int value is emitted", in: counter{N: signatures.Some(0)}, want: `{"n":0}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(out) != tc.want {
				t.Errorf("Marshal = %s, want %s", out, tc.want)
			}
		})
	}
}
