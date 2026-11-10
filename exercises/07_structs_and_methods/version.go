package structs

import "errors"

var (
	// ErrInvalidVersion reports text or components that are not a valid semantic version.
	ErrInvalidVersion = errors.New("invalid version")
	// ErrInvalidPart reports a Part value that is not PartMajor, PartMinor or PartPatch.
	ErrInvalidPart = errors.New("invalid version part")
)

// Part selects which component Version.Bump increments. The zero Part is invalid.
type Part int

const (
	// PartMajor increments the major component and resets minor and patch.
	PartMajor Part = iota + 1
	// PartMinor increments the minor component and resets patch.
	PartMinor
	// PartPatch increments the patch component.
	PartPatch
)

// Version is an immutable semantic version 2.0.0. The zero value is 0.0.0.
//
// Compare, Equal and the Version ordering ignore build metadata. The == operator
// compares every field, including build, so use Equal for semantic equality.
type Version struct {
	major, minor, patch uint64
	prerelease, build   string
}

// NewVersion builds a Version from components. prerelease and build are
// dot-separated identifiers of [0-9A-Za-z-]. Numeric prerelease identifiers
// must not have leading zeros. Empty strings mean none. Invalid input is ErrInvalidVersion.
func NewVersion(major, minor, patch uint64, prerelease, build string) (Version, error) {
	return Version{}, errNotImplemented
}

// ParseVersion parses "[v]MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]". Core numbers
// must not have leading zeros. Anything else is ErrInvalidVersion.
func ParseVersion(text string) (Version, error) { return Version{}, errNotImplemented }

// Major returns the major component.
func (v Version) Major() uint64 { return 0 }

// Minor returns the minor component.
func (v Version) Minor() uint64 { return 0 }

// Patch returns the patch component.
func (v Version) Patch() uint64 { return 0 }

// Prerelease returns the prerelease identifiers, or "".
func (v Version) Prerelease() string { return "" }

// Build returns the build metadata, or "".
func (v Version) Build() string { return "" }

// Bump returns a new version with part incremented and lower components reset
// to zero. Prerelease and build are cleared. Unknown parts are ErrInvalidPart.
func (v Version) Bump(part Part) (Version, error) { return Version{}, errNotImplemented }

// Compare orders versions by semantic-version precedence and returns -1, 0 or +1.
// Build metadata is ignored.
func (v Version) Compare(o Version) int { return 0 }

// Equal reports whether v and o have the same precedence. Build metadata is ignored.
func (v Version) Equal(o Version) bool { return false }

// String returns the canonical form, for example "1.2.3-rc.1+build.5". It never
// includes a leading "v".
func (v Version) String() string { return "" }

// MarshalText implements encoding.TextMarshaler using String.
func (v Version) MarshalText() ([]byte, error) { return nil, errNotImplemented }

// UnmarshalText implements encoding.TextUnmarshaler using ParseVersion. On error
// the receiver is unchanged.
func (v *Version) UnmarshalText(text []byte) error { return errNotImplemented }
