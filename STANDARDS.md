# Coding Standards

How this codebase is built, and why. Rules here come in two strengths,
and the distinction is the most important rule of all:

> **Enforce what causes silent correctness bugs. Document what is an
> architectural judgment call.**
>
> A convention whose violation produces a quiet wrong-answer (a read
> escaping a transaction, a model drifting from the schema) gets
> *machinery* — an analyzer, a generator, a drift check in `bin/check`.
> A convention whose violation is visible erosion (a package in the wrong
> place, a boundary crossed) gets *documentation and review*, because a
> human should weigh whether the deviation is justified.

Everything machine-enforced runs in `bin/check`. If `bin/check` passes,
the enforced rules hold.

## Philosophy

- **Packages are the primary abstraction.** Not structs. A package's
  exported functions are its interface; its file boundary is its
  encapsulation. Don't build "service" structs whose only job is to hold
  dependencies — that's a package wearing a costume.
- **Globals are legitimate for thread-safe, single-instance resources** —
  the connection pool (`db.Conn`), the logger registry. There is exactly
  one of each per process, and threading them through every call adds
  ceremony without information. Anything that isn't both thread-safe and
  genuinely singular does not get to be a global.
- **Dependency injection earns its place at seams.** A parameter exists
  because call sites genuinely vary (pool vs. transaction), never to make
  code abstractly "testable."
- **Abstractions are earned.** A helper, generator, or interface is
  justified after the pattern it captures has been hand-written about
  three times — not before. Speculative infrastructure rots.
- Rails-influenced, but within Go idiom. Deviations from mainstream Go
  are allowed when concretely justified, and the justification belongs in
  this file.

## Layout

- **Flat root.** Packages live at the repository root (plus `web/...` for
  HTTP). No `internal/` tree.
- **Package = domain aggregate.** A thing with its own table and
  lifecycle gets a package. A mere value or projection (a wire shape)
  lives with the handler that shapes it. Don't mint a package per
  struct.
- **Bounded contexts are documented, not compiler-enforced.** When
  related aggregates form a context, the grouping and its boundary are
  declared in package doc comments (`doc.go`) and listed here. Crossing a
  context boundary is a review conversation, not a build error.
- **Mechanism vs. domain.** `db` owns generic data-access mechanism
  (`FindAllTx[T]`, `UpdateTx`, `InTx`, `FilterUnset`). Domain packages
  own their specific SQL. Handlers never import `db` for queries — they
  go through the domain package. (Handlers may import `db` for sentinel
  errors like `db.ErrNotFound`.)
- Extra binaries live in `cmd/` (`cmd/modelgen`, `cmd/txgen`, `cmd/lint`).
  The repo root is the server's `main`.

### Contexts

- (none yet — list each context and its aggregate packages here as they
  appear)

## The transaction pattern (enforced)

The single most important convention. In the project this bootstrap was
extracted from, it produced two shipped bugs while held by discipline
alone — so here it is held by machinery.

- **Hand-write only the `Tx` form:**
  `func IndexByNameTx(ctx context.Context, tx db.Queryable, ...)`.
  Inside a transaction (`db.InTx` / `db.ExecInTx`), every call must be a
  `*Tx` call passing the transaction. There is no ambient transaction —
  nothing rides in `context.Context` — because `pgx.Tx` is not
  concurrency-safe and implicit transactions hide that hazard.
- **Never hand-write the bare form.** `bin/generate` (cmd/txgen) emits
  the pool-backed delegate (`FooByName`) for every exported `FooTx`
  taking a `db.Queryable`. The delegate is the only code that touches the
  pool, and it's machine-written, so it cannot drift.
- **`db.Conn` is confined** (analyzer: `connconfine`). Only generated
  delegates, `db/db.go` (bootstrap), `db/tx.go` (the transaction
  boundary), and `package main` may reference it. Hand-written code
  mid-call-tree takes a `db.Queryable` parameter instead.
- **A `db.Queryable` parameter must be used** (analyzer: `txparam`).
  Accepting `tx` and ignoring it is exactly how reads silently escape
  transactions.

## Code generation (enforced via drift check)

**The schema is the source of truth.** Migrations are hand-authored SQL
(`tern`); the Go model is generated from the live database, never typed
by hand.

- `cmd/modelgen` introspects Postgres (`modelgen.yaml` maps tables to
  packages) and emits the struct, `ToRowMap`, and the
  database-default-aware zero-value filtering. Column comments become
  field docs. Nullability becomes pointer types — the compiler now knows
  what the schema knows.
- `cmd/txgen` emits the pool delegates described above.
- **Generated output is committed.** Generation is a deliberate act:
  `tern migrate && bin/generate` (or `go generate .` — the directives in
  `gen.go` are the source of truth and `bin/generate` delegates to them).
  Generation is *not* wired into builds or the air loop — builds and
  fresh clones never need a database.
- `bin/check` regenerates and fails on drift. Stale generated code is a
  hard failure, not a footgun.
- Generated files carry the standard `// Code generated ... DO NOT EDIT.`
  header; analyzers and lint skip them by that header.
- A generator must stay *boring*: introspect, map, emit. The moment one
  needs runtime cleverness, the cleverness belongs in `db` as mechanism.

## Errors

- **Errors are part of the API contract.** Every handler error is
  `{"message": ..., "status": ...}` via `web/apierror` — a flat shape
  clients can parse and branch on. Internal detail never crosses the
  boundary — `apierror.Internal` logs it and returns a generic 500. If
  this service mimics an existing API, mimic its error shape and status
  codes too; clients parse error bodies.
- **Wrap with `%w`, always.** `fmt.Errorf("saving index %q: %w", name,
  err)`. Use `%v` only to deliberately sever the chain (rare; say why).
- **Sentinels for branching.** `db.ErrNotFound` + `errors.Is`. They live
  in the package that owns the condition. Custom error types wait until
  something must carry data a sentinel can't.

## Testing

- **Integration-first against real Postgres.** The HTTP handler is the
  primary entry point; assert through the API and then through reads, not
  by inspecting internals. Unit tests are for pure logic that's awkward
  to reach through HTTP.
- **Mocks are reserved for external services**, and a substitute must
  *pay rent in more than one context* — e.g. a fake third-party API
  that's also useful for local development. A mock that exists only so a
  test can run fails the bar. Internal seams (`db.Queryable`) exist for
  runtime composition, not for mocking.
- **Isolation = database-per-package.** Every DB-touching package has
  `func TestMain(m *testing.M) { webtest.Main(m) }`, which clones a
  fresh database from `<project>_template` (created by `bin/testdb`,
  re-run after migrations change) and drops it after. `go test ./...`
  runs packages as parallel processes, each on its own clone, one global
  pool each — parallelism without touching the global.
- **Within a package:** a test that touches only rows it created (unique
  fixture names via `testdata`) calls `t.Parallel()`. A test that asserts
  on table-wide state stays serial. That's the whole contract.
- Test-first for behavior changes; one behavior per test; tests survive
  refactors because they only speak through public interfaces.

## Logging

- One named logger per package: `var logger = logging.Logger("mypackage")`.
  Runtime filtering via the settings DSL (`"_all,-db:debug"`).
- No leftover debugging: no commented-out log lines, no exploratory
  queries in committed code. The leveled logger *is* the debug mechanism;
  if a debug line isn't worth keeping at `Debug` level, it isn't worth
  committing.

## Style

- Comments follow the high-signal policy: packages and exported functions
  get one-sentence contracts; inline comments only for what the code
  can't say (constraints, invariants, workarounds). Never narrate the
  code.
- Dot-imports are permitted only for the test DSL (`webtest`) in test
  files, nowhere else.
- `any`, not `interface{}`.
- Match the file you're editing.

## Tooling reference

| Command        | What it does                                                        |
| -------------- | ------------------------------------------------------------------- |
| `bin/check`    | Full gate: fmt, build, vet, custom analyzers, golangci, drift, tests |
| `bin/generate` | Regenerate models (live schema) + delegates (signatures)             |
| `bin/testdb`   | Recreate + migrate the test template database                        |
| `bin/migration`| Create a new tern migration                                          |
| `bin/recreate` | Recreate + migrate the dev database                                  |
| `bin/setup`    | Install toolchain dependencies                                       |

Custom analyzers live in `analyzers/` and run via `go run ./cmd/lint
./...`. Stock linters via `.golangci.yml`. Secrets/config come from `.env`
(gitignored; start from `.env.example`).
