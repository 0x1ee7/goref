package deployment

import "runtime/debug"

// Version and Commit are normally set at link time, which only works on plain
// string variables:
//
//	go build -ldflags "-X goref/exercises/18_deployment.Version=1.2.3"
var (
	Version string
	Commit  string
)

// BuildVersion returns Version when linked in. Otherwise it falls back to the
// module version recorded by the toolchain, then to "dev". A plain `go build`
// records "(devel)", which is not a version, so it is skipped.
func BuildVersion() string {
	if Version != "" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return "dev"
}

// BuildCommit returns Commit when linked in, otherwise the vcs.revision the
// toolchain stamped into the binary, or "" when neither is known.
func BuildCommit() string {
	if Commit != "" {
		return Commit
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				return s.Value
			}
		}
	}
	return ""
}
