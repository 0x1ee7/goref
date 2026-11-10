// Command goref-service is the process entry point. All behaviour lives in
// package deployment, so tests can exercise it in-process.
package main

// main is intentionally empty: wire deployment.Main with a signal-cancelled
// context and os.Exit with its result.
func main() {}
