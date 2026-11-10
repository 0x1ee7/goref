package structs

import (
	"cmp"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	// ErrInvalidVersion reports text or components that are not a valid semantic version.
	ErrInvalidVersion = errors.New("invalid version")
	// ErrInvalidPart reports a Part that Bump does not know.
	ErrInvalidPart = errors.New("invalid version part")
)

// Part selects which component Version.Bump increments. The zero value is not a
// valid Part.
type Part int

const (
	// PartMajor bumps the major component and resets minor and patch.
	PartMajor Part = iota + 1
	// PartMinor bumps the minor component and resets patch.
	PartMinor
	// PartPatch bumps the patch component.
	PartPatch
)

// versionPattern splits "[v]MAJOR.MINOR.PATCH[-PRE][+BUILD]". Core numbers may
// not have leading zeros.
var versionPattern = regexp.MustCompile(
	`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([^+]+))?(?:\+(.+))?$`)

// identPattern matches one dot-separated prerelease identifier. A numeric
// identifier may not have leading zeros.
const identPattern = `(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)`

var (
	prereleasePattern = regexp.MustCompile(`^` + identPattern + `(?:\.` + identPattern + `)*$`)
	buildPattern      = regexp.MustCompile(`^[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*$`)
)

// Version is a semantic version: MAJOR.MINOR.PATCH with optional prerelease
// and build metadata. The zero value is 0.0.0. Fields are unexported, so
// every Version is valid.
//
// Compare and Equal follow SemVer precedence and ignore build metadata. The
// == operator compares every field, including build metadata.
type Version struct {
	major, minor, patch uint64
	prerelease          string
	build               string
}

// NewVersion builds a Version. prerelease and build may be "" for none. Otherwise
// they must match the SemVer identifier rules.
func NewVersion(major, minor, patch uint64, prerelease, build string) (Version, error) {
	if prerelease != "" && !prereleasePattern.MatchString(prerelease) {
		return Version{}, fmt.Errorf("%w: prerelease %q", ErrInvalidVersion, prerelease)
	}
	if build != "" && !buildPattern.MatchString(build) {
		return Version{}, fmt.Errorf("%w: build %q", ErrInvalidVersion, build)
	}
	return Version{major: major, minor: minor, patch: patch, prerelease: prerelease, build: build}, nil
}

// ParseVersion parses a version such as "1.2.3", "v1.2.3-rc.1" or
// "1.0.0+build.5". Leading zeros in core numbers are rejected.
func ParseVersion(text string) (Version, error) {
	m := versionPattern.FindStringSubmatch(text)
	if m == nil {
		return Version{}, fmt.Errorf("%w: %q", ErrInvalidVersion, text)
	}
	var nums [3]uint64
	for i := range nums {
		n, err := strconv.ParseUint(m[i+1], 10, 64)
		if err != nil {
			return Version{}, fmt.Errorf("%w: %q: component out of range", ErrInvalidVersion, text)
		}
		nums[i] = n
	}
	return NewVersion(nums[0], nums[1], nums[2], m[4], m[5])
}

// Major returns the major component.
func (v Version) Major() uint64 { return v.major }

// Minor returns the minor component.
func (v Version) Minor() uint64 { return v.minor }

// Patch returns the patch component.
func (v Version) Patch() uint64 { return v.patch }

// Prerelease returns the prerelease identifiers, without the leading "-".
func (v Version) Prerelease() string { return v.prerelease }

// Build returns the build metadata, without the leading "+".
func (v Version) Build() string { return v.build }

// Bump returns a copy with the chosen component incremented. Lower components
// reset to zero, and prerelease and build metadata are cleared. An unknown Part
// gives ErrInvalidPart.
func (v Version) Bump(p Part) (Version, error) {
	next := Version{}
	switch p {
	case PartMajor:
		if v.major == ^uint64(0) {
			return Version{}, fmt.Errorf("%w: major overflow", ErrInvalidVersion)
		}
		next.major = v.major + 1
	case PartMinor:
		if v.minor == ^uint64(0) {
			return Version{}, fmt.Errorf("%w: minor overflow", ErrInvalidVersion)
		}
		next.major, next.minor = v.major, v.minor+1
	case PartPatch:
		if v.patch == ^uint64(0) {
			return Version{}, fmt.Errorf("%w: patch overflow", ErrInvalidVersion)
		}
		next.major, next.minor, next.patch = v.major, v.minor, v.patch+1
	default:
		return Version{}, fmt.Errorf("%w: %d", ErrInvalidPart, p)
	}
	return next, nil
}

// Compare returns -1, 0 or +1 by SemVer precedence. Build metadata is ignored.
// A release outranks its prereleases. Dot-separated prerelease identifiers
// compare numerically when both are numbers, and otherwise lexically. Numeric
// identifiers rank below alphanumeric ones.
func (v Version) Compare(o Version) int {
	if c := cmp.Compare(v.major, o.major); c != 0 {
		return c
	}
	if c := cmp.Compare(v.minor, o.minor); c != 0 {
		return c
	}
	if c := cmp.Compare(v.patch, o.patch); c != 0 {
		return c
	}
	switch {
	case v.prerelease == o.prerelease:
		return 0
	case v.prerelease == "":
		return 1
	case o.prerelease == "":
		return -1
	}
	return comparePrerelease(v.prerelease, o.prerelease)
}

// comparePrerelease compares two non-empty prerelease strings.
func comparePrerelease(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if c := compareIdentifier(as[i], bs[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(as), len(bs))
}

// compareIdentifier compares one prerelease identifier.
func compareIdentifier(a, b string) int {
	aNum, bNum := isNumeric(a), isNumeric(b)
	switch {
	case aNum && bNum:
		// No leading zeros, so a longer number is larger.
		if c := cmp.Compare(len(a), len(b)); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	case aNum:
		return -1
	case bNum:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

// isNumeric reports whether s is a non-empty string of ASCII digits.
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// Equal reports whether v and o have the same SemVer precedence. Build metadata
// is ignored.
func (v Version) Equal(o Version) bool { return v.Compare(o) == 0 }

// String returns the canonical form "MAJOR.MINOR.PATCH[-PRE][+BUILD]", without a
// leading "v".
func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
	if v.prerelease != "" {
		s += "-" + v.prerelease
	}
	if v.build != "" {
		s += "+" + v.build
	}
	return s
}

// MarshalText implements encoding.TextMarshaler using the String form.
func (v Version) MarshalText() ([]byte, error) {
	return []byte(v.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler. On error the receiver is
// unchanged.
func (v *Version) UnmarshalText(text []byte) error {
	parsed, err := ParseVersion(string(text))
	if err != nil {
		return err
	}
	*v = parsed
	return nil
}
