package deployment

// Version and Commit are set at link time with -ldflags -X, so they must be
// plain string variables:
//
//	go build -ldflags "-X goref/exercises/18_deployment.Version=1.2.3"
var (
	Version string
	Commit  string
)

// BuildVersion returns Version when linked in. Otherwise it falls back to the
// module version recorded by the toolchain (runtime/debug.ReadBuildInfo), then
// to "dev". A plain `go build` records "(devel)", which is not a version.
func BuildVersion() string { return "" }

// BuildCommit returns Commit when linked in, otherwise the vcs.revision the
// toolchain stamped into the binary, or "" when neither is known.
func BuildCommit() string { return "" }
