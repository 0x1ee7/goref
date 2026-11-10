package signatures

import (
	"errors"
)

// ErrInvalidEmail is returned for a malformed email or an attempt to clear it.
var ErrInvalidEmail = errors.New("invalid email")

// ErrInvalidNickname is returned for an empty or over-long nickname.
var ErrInvalidNickname = errors.New("invalid nickname")

// Optional records one of three JSON states for a field: absent (unset),
// explicit null, or a value. The zero value is unset.
type Optional[T any] struct{}

// Some returns an Optional holding v.
func Some[T any](v T) Optional[T] { return Optional[T]{} }

// Null returns an Optional that represents an explicit JSON null.
func Null[T any]() Optional[T] { return Optional[T]{} }

// IsSet reports whether the field was provided, either as a value or as null.
func (o Optional[T]) IsSet() bool { return false }

// IsNull reports whether the field was provided as an explicit null.
func (o Optional[T]) IsNull() bool { return false }

// Get returns the value and true only when the field holds a value.
func (o Optional[T]) Get() (T, bool) {
	var zero T
	return zero, false
}

// IsZero reports whether the field is unset. encoding/json uses it for the
// omitzero tag option.
func (o Optional[T]) IsZero() bool { return false }

// MarshalJSON encodes a value as its JSON form and a null as null.
func (o Optional[T]) MarshalJSON() ([]byte, error) { return []byte("null"), nil }

// UnmarshalJSON decodes JSON null as an explicit null and any other JSON
// value as a set value.
func (o *Optional[T]) UnmarshalJSON(data []byte) error { return nil }

// User is a stored user record. Nickname nil means no nickname.
type User struct {
	ID       int
	Email    string
	Nickname *string
}

// UserPatch is the body of a PATCH request. An absent member leaves the field
// unchanged, null clears it, and a value sets it.
type UserPatch struct {
	Email    Optional[string] `json:"email,omitzero"`
	Nickname Optional[string] `json:"nickname,omitzero"`
}

// ApplyPatch returns a patched copy of u and never modifies u or the memory it
// points to. Email cannot be cleared: a null email is ErrInvalidEmail. A
// nickname that is null clears it. Other values are validated per the spec.
func ApplyPatch(u User, p UserPatch) (User, error) {
	return User{}, nil
}
