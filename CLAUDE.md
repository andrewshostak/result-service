# Project context

- Go service with hexagonal architecture.
- See README.md for the domain model for the state before migration.
- An active migration plan is in progress: migration-plan.md
- DB schema evolution history: db-changes.md

# Working conventions

## Running tests

Unit tests:
```
make unit-tests
```

Functional tests (require Docker):
```
make functional-tests
```
