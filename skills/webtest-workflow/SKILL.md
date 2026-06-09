---
name: webtest-workflow
description: Use when writing DB-backed Go tests, fixtures, or factories. Tests hit a real per-package Postgres clone via webtest.Main — no mocks for ordinary query/command behavior.
---

# Webtest Workflow

## Default To Real Postgres

Integration-first: tests run against a real database cloned from the
migrated template, entered through the same code paths production uses.
Do not add mocking or interface-injection ceremony around ordinary DB
behavior. Mocks are reserved for external services, and only when the fake
pays rent in more than one context.

Use two tiers:

- Pure unit tests for pure functions with literal inputs (parsing,
  formatting, partition behavior, etc.).
- Real-DB integration tests for queries, commands, and handler flows.

## Test Setup

Every DB-touching package needs exactly one TestMain:

```go
func TestMain(m *testing.M) { webtest.Main(m) }
```

`webtest.Main` clones `<project>_template` into `<project>_test_<pid>`,
points `db.Conn` at it, runs the package's tests, and drops the clone.
Each package is a separate process with its own database — packages are
isolated from each other and from dev data, and parallelize freely.

Run `bin/testdb` once after checkout and after migration changes to rebuild
the template. In a worktree, `TEMPLATE_DB` (written by `bin/worktree-setup`)
points the harness at the worktree's own template clone.

## Within a package

- Tests that only touch rows they created (unique fixture names) call
  `t.Parallel()`.
- Tests that assert on table-wide state stay serial.
- There is no per-test rollback — the database lives for the package run.
  Use unique names (`fmt.Sprintf("user-%d@example.com", i)` or
  `fixtureid.For(table, name)`) instead of assuming a clean table.

## Factories

`auth.Make(ctx, opts...)` inserts a user with unique random-tagged
defaults; `auth.MakePassword` is the known cleartext for UI signin flows.
Options (`auth.WithEmail`, `WithRoles`, ...) override the particulars.
New aggregates grow a `Make` of the same shape — the factory's uniqueness
is what lets callers use `t.Parallel()`.

## HTTP tests

- JSON handler tests use `webtest.Request[T]` with a bare handler and
  `webtest.Path(...)` for params.
- Server-rendered flows (sessions, redirects, HTML) use `webtest.Server(ctx)`
  for a production-wired Echo app and `webtest.NewClient` for a
  cookie-carrying client, so signin state persists across requests.

## Persistence in tests

The transaction rules apply in tests too: call bare `Foo` forms, or open
`db.InTx` / `db.ExecInTx` and pass the tx to `FooTx` forms explicitly.
Never reference `db.Conn` in test code — `webtest.Main` has already wired
it.
