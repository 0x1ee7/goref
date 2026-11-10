# 18 Deployment

## Scenario

You own `goref-service`, a small HTTP API that is about to ship to a container
platform. Before it can go to production it needs to:

- refuse to start with a bad environment, and report every problem at once;
- expose health, readiness, and version endpoints for the orchestrator;
- drain in-flight requests on SIGTERM instead of dropping them;
- offer a CLI (`serve`, `check-config`, `version`) that ops can run anywhere;
- carry its version and commit into the binary at build time;
- ship in a minimal, non-root container image.

Each part below is one of those concerns. The logic lives in the package root
(`package deployment`). `cmd/goref-service` is a thin `main` that wires signals
to `Main`.

## Parts

### Part 1: Configuration from the environment

`LoadConfig(getenv func(string) string) (Config, error)`

Variables:

| Variable       | Required | Default       | Notes                                              |
|----------------|----------|---------------|----------------------------------------------------|
| `DATABASE_URL` | yes      | -             | scheme must be `postgresql` or `sqlite`            |
| `API_TOKEN`    | yes      | -             | at least 16 characters; must be a `Secret`         |
| `APP_NAME`     | no       | `goref`       |                                                    |
| `PORT`         | no       | `8080`        | integer 1–65535                                    |
| `DEBUG`        | no       | `false`       | accepts `1/0`, `t/f`, `true/false`, `yes/no`, `on/off` (case-insensitive) |
| `ALLOWED_HOSTS`| no       | empty         | comma-separated; blanks dropped, entries trimmed   |
| `REQUEST_TIMEOUT` | no    | `30s`         | a `time.Duration`, or bare seconds (`45`)          |
| `LOG_LEVEL`    | no       | `info`        | `debug`, `info`, `warn`, `error`; case-insensitive |

Edge cases:

- Blank or whitespace-only values count as unset.
- Missing required values, invalid values, and out-of-range values are all
  reported. Return them joined with `errors.Join`, each as a `*FieldError`
  (`Var`, `Err`), so the caller sees every problem at once.
- `ErrRequired` and `ErrInvalid` are sentinels; test with `errors.Is`.
- `Secret` must never leak: `%v`, `%+v`, `%#v`, and `slog` all print `***`.
- `RedactURL` removes the password from a database URL but keeps the rest.

### Part 2: HTTP server

`NewServer(cfg Config, deps Deps) *http.Server`

- `GET /healthz`: liveness. Always 200 `{"status":"ok"}` while the process runs.
- `GET /readyz`: runs every `Deps.Readiness` check with a 2s timeout each. 200 when
  all pass; otherwise 503 `{"status":"not_ready","checks":{...}}` with each failing
  check's `"error: <msg>"`. Each check receives a context with a deadline.
- `GET /version`: `{"version":..., "commit":...}`. A missing commit is `null`.
- Unknown paths: JSON 404. Wrong method on a known route: 405 with
  `Allow: GET, HEAD`.
- Hardening: `ReadHeaderTimeout` 5s, `ReadTimeout` and `WriteTimeout` from
  `RequestTimeout`, `IdleTimeout` 2m, `MaxHeaderBytes` 64 KiB, `Addr` `:PORT`.

### Part 3: Graceful shutdown

`Run(ctx context.Context, srv *http.Server, ln net.Listener, shutdownTimeout time.Duration) error`

- Serve on `ln` until `ctx` is cancelled, then call `Shutdown` and let in-flight
  requests finish.
- Cancelled context with a clean drain returns `nil`. A new connection after
  return is refused.
- A drain that exceeds `shutdownTimeout` calls `Close` and returns an error that
  wraps `context.DeadlineExceeded`.
- A serve failure (for example, a closed listener) returns that error. It is not
  `http.ErrServerClosed`.
- An idle server stops promptly; it does not wait out the grace period.

Note the `context.WithoutCancel` trap: the shutdown context must not already be
cancelled, or `Shutdown` returns immediately.

### Part 4: Command-line interface

`Main(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int`

Exit codes: `0` success, `1` runtime failure, `2` usage or configuration error.

- `serve [-addr ADDR]`: validate config (exit 2 on failure, starting nothing),
  log JSON to stderr via `slog`, print `listening on ADDR` to stdout once the
  listener is bound, run until `ctx` is cancelled, and exit 0. A listen failure
  exits 1. The API token must never reach stderr.
- `check-config`: print `configuration ok` on success; on failure print one
  `error: ...` line per problem and exit 2.
- `version`: print `goref-service VERSION`, plus ` (commit SHA)` when the commit
  is set.
- `help`, `-h`: usage to stdout, exit 0.
- No arguments, an unknown command, or stray arguments: usage to stderr, exit 2.

### Part 5: Build metadata

`Version` and `Commit` are package-level string variables that `go build` sets
with `-ldflags "-X ..."`. `BuildVersion()` and `BuildCommit()` fall back to
`runtime/debug.ReadBuildInfo` when the variables are empty.

- Use `-trimpath` for reproducible builds.
- Skip the `(devel)` module version and fall back to `dev`.
- `BuildCommit` falls back to the `vcs.revision` build setting.

The build test runs a real `go build` with injected ldflags and checks the
binary's `version` output. Under `tools/solutions` it builds the reference
solution through its own overlay, because subprocess builds ignore the harness
overlay.

### Part 6: Container image

`Dockerfile` and `.dockerignore` are text artifacts, not Go code. The lint test
checks:

- a `golang:1.27` builder stage that is pinned, with no `latest` tags anywhere;
- a runtime stage that is `gcr.io/distroless/static-debian12:nonroot` or `scratch`;
- `CGO_ENABLED=0` in the builder, and the binary copied with `COPY --from=`;
- a non-root `USER` in the final stage;
- an `ENTRYPOINT` in the final stage, with `ENTRYPOINT` and `CMD` in JSON exec form;
- `.dockerignore` excludes `.git` and build output (`bin/` or `dist/`).

## Concepts exercised

- Environment parsing, `errors.Join`, sentinel errors, and typed errors with `Unwrap`.
- Secret types: `String`, `GoString`, and `slog.LogValuer` to prevent leaks.
- `net/http` server configuration: timeouts, method routing, JSON errors.
- Context cancellation, `Shutdown` vs `Close`, `context.WithoutCancel`.
- Injected `getenv`, `io.Writer`, and `context` for testable `main` logic.
- `runtime/debug.ReadBuildInfo` and `-ldflags -X` version injection.
- Multi-stage Docker builds, distroless non-root images, and `.dockerignore`.

## Run


```sh
go test ./exercises/18_deployment/
go test ./exercises/18_deployment/ -run Part2 -v
go run ./tools/solutions ./exercises/18_deployment/
```

Recommended on Linux or macOS: `go test -race ./exercises/18_deployment/`.

## Stretch (untested)

- Add `SIGHUP` to reload configuration without a restart.
- Expose Prometheus metrics on a separate admin port.
- Add a `/debug/pprof` mount that only listens on loopback.
- Replace the text lint with a parser for `docker buildx bake` output.
- Build the binary for several platforms with `GOOS`/`GOARCH` and check each one.
