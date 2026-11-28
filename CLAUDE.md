# Project context

- Go service with hexagonal architecture.
- See README.md for the domain model.

# Working conventions

## Go toolchain

The `go` resolved via `PATH` may be a newer Homebrew install (e.g. `/opt/homebrew/bin/go`) than
the official Go install at `/usr/local/go/bin/go`, which matches the version pinned in `go.mod`'s
`go` directive. Go's toolchain auto-switching (`GOTOOLCHAIN=auto`) doesn't fix this - it only
switches to a *newer* toolchain when the resolved one is too old, never down to match a lower
pinned version. Tools that read Go's export data (e.g. `mockery`) can fail with confusing internal
errors when run under a too-new toolchain.

Run `go`, `mockery`, or similar Go-version-sensitive commands via `/usr/local/go/bin/go` /
`PATH="/usr/local/go/bin:$PATH" mockery ...` directly, rather than relying on whatever `go` is
first on `PATH`.

## Code organization

Within a Go file, order top-level declarations as: public (exported) methods first, then private
(unexported) methods, then standalone functions (not methods on a type) last.

This applies to files built around a type with real behavior (service, repository, handler, etc.).
It doesn't apply to `model.go`-style files, where structs and their own methods (e.g. `ToDomain()`)
are declared together.

## Comments

No multi-line comments on functions/methods. Many need no comment at all - a well-named
function doesn't need one. When a comment is warranted, one line max, and it must explain a
non-obvious *why* or a tricky detail, never restate *what* the code already says.

## Test files

Unit tests: one test file per implementation file (`foo.go` → `foo_test.go`), never split a
file's tests across multiple test files. Add new test functions to the existing `_test.go` file.

Functional tests (`functionaltests/`): one file per triggered endpoint/scenario, not per
implementation file - e.g. `trigger_result_check_test.go` and
`trigger_subscriber_notification_test.go` both test `trigger.go`, split by endpoint.

When unit testing a gin handler or middleware, prefer `gin.CreateTestContext` and call the
`gin.HandlerFunc` directly over spinning up a real `gin.Engine`/router. Only reach for a real
engine (`gin.New()` + `r.Use()`/`r.GET()` + `r.ServeHTTP()`) when the behavior under test genuinely
requires a real handler chain (e.g. panic recovery, goroutine-based timeout middleware).

## Running tests

Unit tests:
```
make unit-tests
```

Functional tests (require Docker):
```
make functional-tests
```
