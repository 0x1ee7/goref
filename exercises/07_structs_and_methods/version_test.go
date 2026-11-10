package structs_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	structs "goref/exercises/07_structs_and_methods"
)

// mustVersion parses text with ParseVersion and fails the test on error.
func mustVersion(t *testing.T, text string) structs.Version {
	t.Helper()
	v, err := structs.ParseVersion(text)
	if err != nil {
		t.Fatalf("ParseVersion(%q): unexpected error: %v", text, err)
	}
	return v
}

func TestPart5Parse(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		tests := []struct {
			in, want, pre, build string
			major, minor, patch  uint64
		}{
			{"1.2.3", "1.2.3", "", "", 1, 2, 3},
			{"v1.2.3", "1.2.3", "", "", 1, 2, 3},
			{"0.0.0", "0.0.0", "", "", 0, 0, 0},
			{"10.20.30", "10.20.30", "", "", 10, 20, 30},
			{"1.0.0-alpha.1+build.5", "1.0.0-alpha.1+build.5", "alpha.1", "build.5", 1, 0, 0},
			{"v2.0.0-rc.1", "2.0.0-rc.1", "rc.1", "", 2, 0, 0},
			{"1.0.0+001", "1.0.0+001", "", "001", 1, 0, 0},
		}
		for _, tt := range tests {
			t.Run(tt.in, func(t *testing.T) {
				_ = mustVersion(t, "1.0.0")
				v, err := structs.ParseVersion(tt.in)
				if err != nil {
					t.Fatalf("ParseVersion(%q): %v", tt.in, err)
				}
				if v.Major() != tt.major || v.Minor() != tt.minor || v.Patch() != tt.patch {
					t.Errorf("components = %d.%d.%d, want %d.%d.%d", v.Major(), v.Minor(), v.Patch(), tt.major, tt.minor, tt.patch)
				}
				if v.Prerelease() != tt.pre || v.Build() != tt.build {
					t.Errorf("pre=%q build=%q, want %q and %q", v.Prerelease(), v.Build(), tt.pre, tt.build)
				}
				if v.String() != tt.want {
					t.Errorf("String() = %q, want %q", v.String(), tt.want)
				}
			})
		}
	})

	t.Run("invalid", func(t *testing.T) {
		inputs := []string{
			"", "1", "1.2", "1.2.3.4", "01.2.3", "1.02.3", "1.2.03",
			"1.2.3-", "1.2.3-01", "1.2.3-a..b", "1.2.3+", "1.2.3+a..b",
			"vv1.2.3", "1.2.-3", "-1.2.3", " 1.2.3", "1.2.3 ", "x.y.z",
			"18446744073709551616.0.0",
		}
		for _, in := range inputs {
			t.Run(fmt.Sprintf("%q", in), func(t *testing.T) {
				_ = mustVersion(t, "1.0.0")
				if _, err := structs.ParseVersion(in); !errors.Is(err, structs.ErrInvalidVersion) {
					t.Errorf("ParseVersion(%q) error = %v, want ErrInvalidVersion", in, err)
				}
			})
		}
	})

	t.Run("max uint64 components", func(t *testing.T) {
		v := mustVersion(t, "18446744073709551615.0.0")
		if v.Major() != 18446744073709551615 {
			t.Errorf("Major() = %d, want max uint64", v.Major())
		}
	})
}

func TestPart5NewVersion(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		_ = mustVersion(t, "1.0.0")
		v, err := structs.NewVersion(1, 2, 3, "rc.1", "build.9")
		if err != nil {
			t.Fatalf("NewVersion: %v", err)
		}
		if v.String() != "1.2.3-rc.1+build.9" {
			t.Errorf("String() = %q, want 1.2.3-rc.1+build.9", v.String())
		}
	})

	t.Run("zero value strings mean none", func(t *testing.T) {
		_ = mustVersion(t, "1.0.0")
		v, err := structs.NewVersion(4, 5, 6, "", "")
		if err != nil {
			t.Fatalf("NewVersion: %v", err)
		}
		if v.String() != "4.5.6" || v.Prerelease() != "" || v.Build() != "" {
			t.Errorf("got %q pre=%q build=%q", v.String(), v.Prerelease(), v.Build())
		}
	})

	t.Run("invalid identifiers", func(t *testing.T) {
		tests := []struct{ pre, build string }{
			{"rc..1", ""},
			{"01", ""},
			{"rc_1", ""},
			{"", "meta+data"},
			{"", "a..b"},
		}
		for _, tt := range tests {
			t.Run(fmt.Sprintf("pre=%q build=%q", tt.pre, tt.build), func(t *testing.T) {
				_ = mustVersion(t, "1.0.0")
				if _, err := structs.NewVersion(1, 0, 0, tt.pre, tt.build); !errors.Is(err, structs.ErrInvalidVersion) {
					t.Errorf("error = %v, want ErrInvalidVersion", err)
				}
			})
		}
	})
}

func TestPart5ZeroValue(t *testing.T) {
	t.Run("zero value is 0.0.0", func(t *testing.T) {
		_ = mustVersion(t, "0.0.0")
		var v structs.Version
		if v.String() != "0.0.0" || !v.Equal(mustVersion(t, "0.0.0")) {
			t.Errorf("zero Version = %q, want 0.0.0", v.String())
		}
	})

	t.Run("zero value is usable in comparisons", func(t *testing.T) {
		_ = mustVersion(t, "0.0.1")
		var v structs.Version
		if v.Compare(mustVersion(t, "0.0.1")) != -1 {
			t.Errorf("0.0.0 should be less than 0.0.1")
		}
	})
}

func TestPart5Bump(t *testing.T) {
	base := mustVersion(t, "1.2.3-rc.1+build.5")
	tests := []struct {
		name string
		part structs.Part
		want string
	}{
		{"major resets minor and patch", structs.PartMajor, "2.0.0"},
		{"minor resets patch", structs.PartMinor, "1.3.0"},
		{"patch", structs.PartPatch, "1.2.4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := base.Bump(tt.part)
			if err != nil {
				t.Fatalf("Bump: %v", err)
			}
			if got.String() != tt.want {
				t.Errorf("Bump = %q, want %q", got.String(), tt.want)
			}
		})
	}

	t.Run("receiver is unchanged", func(t *testing.T) {
		if _, err := base.Bump(structs.PartMajor); err != nil {
			t.Fatalf("Bump: %v", err)
		}
		if base.String() != "1.2.3-rc.1+build.5" {
			t.Errorf("receiver changed to %q", base.String())
		}
	})

	t.Run("invalid part", func(t *testing.T) {
		for _, p := range []structs.Part{0, 4, -1} {
			t.Run(fmt.Sprint(int(p)), func(t *testing.T) {
				_ = mustVersion(t, "1.0.0")
				if _, err := base.Bump(p); !errors.Is(err, structs.ErrInvalidPart) {
					t.Errorf("Bump(%d) error = %v, want ErrInvalidPart", p, err)
				}
			})
		}
	})
}

func TestPart5Precedence(t *testing.T) {
	t.Run("spec ordering", func(t *testing.T) {
		// Each entry is strictly lower than the next one.
		ordered := []string{
			"1.0.0-alpha",
			"1.0.0-alpha.1",
			"1.0.0-alpha.beta",
			"1.0.0-beta",
			"1.0.0-beta.2",
			"1.0.0-beta.11",
			"1.0.0-rc.1",
			"1.0.0",
			"1.0.1",
			"1.1.0",
			"1.10.0",
			"2.0.0",
		}
		for i := range len(ordered) - 1 {
			lo := mustVersion(t, ordered[i])
			hi := mustVersion(t, ordered[i+1])
			if got := lo.Compare(hi); got != -1 {
				t.Errorf("Compare(%s, %s) = %d, want -1", lo, hi, got)
			}
			if got := hi.Compare(lo); got != 1 {
				t.Errorf("Compare(%s, %s) = %d, want 1", hi, lo, got)
			}
		}
	})

	t.Run("numeric components compare as numbers", func(t *testing.T) {
		nine := mustVersion(t, "1.9.0")
		ten := mustVersion(t, "1.10.0")
		if nine.Compare(ten) != -1 {
			t.Errorf("1.9.0 should sort before 1.10.0")
		}
	})

	t.Run("equal precedence", func(t *testing.T) {
		v := mustVersion(t, "1.0.0")
		if v.Compare(mustVersion(t, "1.0.0")) != 0 {
			t.Errorf("Compare of equal versions != 0")
		}
	})

	t.Run("build metadata is ignored", func(t *testing.T) {
		a := mustVersion(t, "1.0.0+aaa")
		b := mustVersion(t, "1.0.0+bbb")
		if a.Compare(b) != 0 {
			t.Errorf("Compare ignored build: got %d, want 0", a.Compare(b))
		}
		if !a.Equal(b) {
			t.Errorf("Equal ignored build: got false, want true")
		}
		if a == b {
			t.Errorf("== reports builds equal; it compares every field")
		}
	})
}

func TestPart5Text(t *testing.T) {
	t.Run("MarshalText canonical form", func(t *testing.T) {
		v := mustVersion(t, "v1.2.3-rc.1+b.5")
		got, err := v.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText: %v", err)
		}
		if string(got) != "1.2.3-rc.1+b.5" {
			t.Errorf("MarshalText = %q", got)
		}
	})

	t.Run("UnmarshalText error keeps receiver", func(t *testing.T) {
		v := mustVersion(t, "2.0.0")
		if err := v.UnmarshalText([]byte("2.x")); !errors.Is(err, structs.ErrInvalidVersion) {
			t.Errorf("error = %v, want ErrInvalidVersion", err)
		}
		if v.String() != "2.0.0" {
			t.Errorf("receiver changed to %q after failed unmarshal", v.String())
		}
	})

	t.Run("JSON round trip", func(t *testing.T) {
		type release struct {
			Version structs.Version `json:"version"`
		}
		in := release{Version: mustVersion(t, "3.1.4-beta.2")}
		raw, err := json.Marshal(in)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if string(raw) != `{"version":"3.1.4-beta.2"}` {
			t.Errorf("json = %s", raw)
		}
		var out release
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if !out.Version.Equal(in.Version) {
			t.Errorf("round trip = %s, want %s", out.Version, in.Version)
		}
	})
}
