package signatures

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ErrInvalidEmail reports an email that is null, malformed, or unparsable.
var ErrInvalidEmail = errors.New("invalid email")

// ErrInvalidNickname reports a nickname that is blank or longer than 32 characters.
var ErrInvalidNickname = errors.New("invalid nickname")

const maxNicknameRunes = 32

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Optional distinguishes three states for a PATCH field: absent (unset), an
// explicit JSON null, and a present value. The zero value is unset.
type Optional[T any] struct {
	value T
	set   bool
	null  bool
}

// Some returns a set Optional holding v. Some(zero) is still set.
func Some[T any](v T) Optional[T] {
	return Optional[T]{value: v, set: true}
}

// Null returns an Optional that is set to an explicit null.
func Null[T any]() Optional[T] {
	return Optional[T]{set: true, null: true}
}

// IsSet reports whether the field was present in the input, as a value or null.
func (o Optional[T]) IsSet() bool { return o.set }

// IsNull reports whether the field was present and explicitly null.
func (o Optional[T]) IsNull() bool { return o.null }

// Get returns the value and true only when the field holds a value.
func (o Optional[T]) Get() (T, bool) {
	if !o.set || o.null {
		var zero T
		return zero, false
	}
	return o.value, true
}

// IsZero reports whether the field is unset. It is used by encoding/json for
// the omitzero option, so Some(zero) and Null are not dropped.
func (o Optional[T]) IsZero() bool { return !o.set }

// MarshalJSON encodes a value as its JSON form, and null or unset as null.
func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if !o.set || o.null {
		return []byte("null"), nil
	}
	return json.Marshal(o.value)
}

// UnmarshalJSON decodes a JSON null as Null, and any other value as Some.
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*o = Null[T]()
		return nil
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*o = Some(v)
	return nil
}

// User is the stored account record.
type User struct {
	ID       int
	Email    string
	Nickname *string
}

// UserPatch is a partial update. Absent fields are left alone. Email can only
// be set, never cleared. Nickname can be cleared with null.
type UserPatch struct {
	Email    Optional[string] `json:"email,omitzero"`
	Nickname Optional[string] `json:"nickname,omitzero"`
}

// ApplyPatch returns a new User with p applied to u. It never mutates u or
// aliases its Nickname pointer, so the caller's record is unchanged on success
// and on error.
func ApplyPatch(u User, p UserPatch) (User, error) {
	out := u
	if u.Nickname != nil {
		out.Nickname = new(*u.Nickname)
	}

	if p.Email.IsSet() {
		raw, ok := p.Email.Get()
		if !ok {
			return User{}, fmt.Errorf("%w: email cannot be null", ErrInvalidEmail)
		}
		email := strings.ToLower(strings.TrimSpace(raw))
		if !emailPattern.MatchString(email) {
			return User{}, fmt.Errorf("%w: %q", ErrInvalidEmail, raw)
		}
		out.Email = email
	}

	if p.Nickname.IsSet() {
		if p.Nickname.IsNull() {
			out.Nickname = nil
		} else {
			raw, _ := p.Nickname.Get()
			nick := strings.TrimSpace(raw)
			if nick == "" {
				return User{}, fmt.Errorf("%w: must not be blank", ErrInvalidNickname)
			}
			if utf8.RuneCountInString(nick) > maxNicknameRunes {
				return User{}, fmt.Errorf("%w: longer than %d characters", ErrInvalidNickname, maxNicknameRunes)
			}
			out.Nickname = new(nick)
		}
	}
	return out, nil
}
