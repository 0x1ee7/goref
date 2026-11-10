package deployment

import (
	"context"
	"io"
)

// Main runs the command line and returns the process exit code. It never calls
// os.Exit. ctx is cancelled by the caller on SIGINT/SIGTERM and stops serve.
//
// Exit codes: 0 success, 1 runtime failure (cannot listen, serve error,
// shutdown overran), 2 usage error or invalid configuration.
//
// Commands: serve [-addr A], check-config, version; help (or -h) prints usage
// to stdout. No arguments, an unknown command, or stray positional arguments
// print usage to stderr and return 2.
//
// serve loads configuration, listens (on -addr, else :PORT), logs JSON to
// stderr at cfg.LogLevel and serves until ctx is done.
// check-config prints "configuration ok", or one "error: ..." line per problem
// to stderr and returns 2.
// version prints "goref-service <version>" plus " (commit <sha>)" when known.
func Main(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	return 1
}
