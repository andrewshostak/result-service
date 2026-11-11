# Project context

- Go service with hexagonal architecture.
- See README.md for the domain model.

# Working conventions

## Code organization

Within a Go file, order top-level declarations as: public (exported) methods first, then private
(unexported) methods, then standalone functions (not methods on a type) last.

## Comments

No multi-line comments on functions/methods. Many need no comment at all - a well-named
function doesn't need one. When a comment is warranted, one line max, and it must explain a
non-obvious *why* or a tricky detail, never restate *what* the code already says.

## Test files

One test file per implementation file (`foo.go` → `foo_test.go`), never split a file's tests
across multiple test files. Add new test functions to the existing `_test.go` file.

## Running tests

Unit tests:
```
make unit-tests
```

Functional tests (require Docker):
```
make functional-tests
```
