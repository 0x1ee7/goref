// Command goref-service is the process entry point. All behaviour lives in
// package deployment, so tests can exercise it in-process.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"goref/exercises/18_deployment"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := deployment.Main(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
