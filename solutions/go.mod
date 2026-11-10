// solutions/ is its own (never built) module so `go build ./...`, `go vet ./...`
// and gopls in the main module skip it. Its files are only compiled through the
// overlay that `go run ./tools/solutions` installs onto exercises/<topic>/,
// which is why they may import exercise-internal packages.
module goref/solutions

go 1.27
