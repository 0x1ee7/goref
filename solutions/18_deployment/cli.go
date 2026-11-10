package deployment

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"time"
)

const (
	program       = "goref-service"
	shutdownGrace = 10 * time.Second

	exitOK      = 0
	exitFailure = 1 // runtime failure: cannot listen, serve error, shutdown overran
	exitUsage   = 2 // bad command line or invalid configuration
)

// Main runs the command line and returns the process exit code. It never calls
// os.Exit, and the caller decides what ctx means: cmd/goref-service cancels it on
// SIGINT/SIGTERM, which stops serve.
//
// Commands: serve [-addr A], check-config, version; help prints usage.
func Main(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return exitUsage
	}
	switch args[0] {
	case "serve":
		return serveCmd(ctx, args[1:], getenv, stdout, stderr)
	case "check-config":
		return checkConfigCmd(args[1:], getenv, stdout, stderr)
	case "version":
		return versionCmd(args[1:], stdout, stderr)
	case "help", "-h", "-help", "--help":
		usage(stdout)
		return exitOK
	default:
		fmt.Fprintf(stderr, "%s: unknown command %q\n\n", program, args[0])
		usage(stderr)
		return exitUsage
	}
}

func serveCmd(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	fs := newFlagSet("serve", stderr)
	addr := fs.String("addr", "", "listen address; default :PORT from configuration")
	if code, ok := parseFlags(fs, args, stderr); !ok {
		return code
	}

	cfg, err := LoadConfig(getenv)
	if err != nil {
		reportConfigErr(stderr, err)
		return exitUsage
	}

	listen := *addr
	if listen == "" {
		listen = ":" + strconv.Itoa(cfg.Port)
	}
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", program, err)
		return exitFailure
	}
	fmt.Fprintf(stdout, "listening on %s\n", ln.Addr())

	logger := slog.New(slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	logger.Info("starting", "config", cfg, "addr", ln.Addr().String())

	if err := Run(ctx, NewServer(cfg, Deps{}), ln, shutdownGrace); err != nil {
		logger.Error("server stopped with error", "err", err)
		return exitFailure
	}
	logger.Info("stopped")
	return exitOK
}

func checkConfigCmd(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	fs := newFlagSet("check-config", stderr)
	if code, ok := parseFlags(fs, args, stderr); !ok {
		return code
	}
	if _, err := LoadConfig(getenv); err != nil {
		reportConfigErr(stderr, err)
		return exitUsage
	}
	fmt.Fprintln(stdout, "configuration ok")
	return exitOK
}

func versionCmd(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("version", stderr)
	if code, ok := parseFlags(fs, args, stderr); !ok {
		return code
	}
	if commit := BuildCommit(); commit != "" {
		fmt.Fprintf(stdout, "%s %s (commit %s)\n", program, BuildVersion(), commit)
	} else {
		fmt.Fprintf(stdout, "%s %s\n", program, BuildVersion())
	}
	return exitOK
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(program+" "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

// parseFlags parses args for a subcommand that takes no positional arguments.
// It reports the exit code and whether the command should continue.
func parseFlags(fs *flag.FlagSet, args []string, stderr io.Writer) (int, bool) {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK, false
		}
		return exitUsage, false
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "%s: unexpected arguments: %v\n", fs.Name(), fs.Args())
		return exitUsage, false
	}
	return 0, true
}

// reportConfigErr prints one "error: ..." line per problem.
func reportConfigErr(w io.Writer, err error) {
	entries := []error{err}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		entries = joined.Unwrap()
	}
	for _, e := range entries {
		fmt.Fprintf(w, "error: %v\n", e)
	}
}

func usage(w io.Writer) {
	fmt.Fprintf(w, `usage: %s <command> [flags]

commands:
  serve          run the HTTP server until SIGINT or SIGTERM (flags: -addr)
  check-config   validate configuration from the environment
  version        print the build version

exit codes: 0 ok, 1 runtime failure, 2 usage or configuration error
`, program)
}
