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
  the connection pool (`db.Conn`), the logger registry, the session
  manager (`web.Sessions`). There is exactly one of each per process, and
  threading them through every call adds ceremony without information.
  Anything that isn't both thread-safe and genuinely singular does not get
  to be a global.
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
  HTTP and `views/` for templ components). No `internal/` tree.
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
  boundary), `package main`, and the `webtest` harness (the composition
  root for tests) may reference it. Hand-written code mid-call-tree takes
  a `db.Queryable` parameter instead.
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
- **Server-rendered handlers branch the same way.** A domain sentinel that
  maps to a non-500 page is handled inline (`errors.Is` → `SetFlash` +
  re-render); everything else propagates to Echo's error handling.

## Server rendering

- **Views render already-shaped data.** `views/` owns `.templ` sources,
  generated `*_templ.go`, render DTOs, and `views.LayoutData`. No DB
  calls, sessions, or domain commands inside `views/` — translation
  happens in the handler (or a `web/*_pages.go` assembler once it grows).
- **Render through `web.RenderPage`** (or `RenderPageData` when the
  component needs the layout data itself). It buffers the whole page
  before writing, so a mid-stream component failure is a 500, not a
  partial 200, and it attaches `User` and `Flash` automatically.
- **Sessions are scs + pgxstore** (the `sessions` table), configured once
  at boot via `web.ConfigureSessions`. Flash is one-shot session state:
  `web.SetFlash` / popped by the next render.
- **Auth is enumeration-safe by construction.** `auth.Authenticate`
  collapses unknown-email and bad-password into `ErrInvalidCredentials`;
  bcrypt cost is 12 (signin is rare; ~100ms is tolerable); the session id
  rotates on signin (`Sessions.RenewToken`) against fixation.
- **Cross-origin POSTs are rejected** by the `web.SameOriginPost`
  middleware (Origin, falling back to Sec-Fetch-Site), and POST→GET
  redirects funnel through `web.SafeRedirect`, which forces local paths.
- **Errors render as pages for browsers.** The router's error handler
  content-negotiates: templ error page for `Accept: text/html` (full
  detail plus a copy button in development only), `apierror` JSON
  otherwise. Internal detail never reaches a production response.
- **Static assets** live in `public/`, served at `/public` (override the
  root with `PUBLIC_DIR`). The shipped `app.css` is a deliberately minimal
  baseline.
- **Process configuration goes through `env.Load()`** — read once in
  main, validated, passed as values. `web.Configure(pool, appEnv)` derives
  cookie security and the error-page posture from it.
- **Signin is throttled** per (client IP, email); bcrypt slows offline
  cracking, the throttle slows online guessing. The default
  `web.SigninThrottle` is in-memory — scale-out replaces it at boot with a
  `web.ThrottleStore` backed by shared state.
- **The design system is the [gesso](https://github.com/mbriggs/gesso)
  module** — templ components, class helpers, and embedded assets served
  at `/ui` (`import "github.com/mbriggs/gesso/ui"`, pinned in go.mod).
  Browse every component and state at `/design` (hidden in production).
  Pages compose `ui` components rather than inventing markup. For local
  component work, check out gesso as a sibling and `go work init . ../gesso`
  (go.work is gitignored); changes land there against gesso's own checks,
  including its dead-CSS linter, then get tagged and bumped here.
- `*_templ.go` is generated output like any other: committed, never
  hand-edited, drift-checked by `bin/check` (`bin/generate` runs
  `templ generate` first — txgen type-checks packages, so templ output
  must be current).

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
- **Factories make test actors.** `auth.Make(ctx, opts...)` inserts a
  user with unique random-tagged defaults (`auth.MakePassword` is the
  known cleartext); aggregates grow a `Make` of the same shape. Uniqueness
  is what keeps parallel tests row-scoped.
- **Session-bound flows test end to end.** `webtest.Server(ctx)` builds a
  production-wired Echo app against the package's test database;
  `webtest.NewClient` carries cookies between requests, so signin, flash,
  and redirects are asserted through real round trips.
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

## Agent configuration

Instruction files for coding agents are generated, not hand-edited. Edit
`ai/common/*.md` (always-loaded invariants) and `ai/rules/*.md`
(path-scoped rules), then `bin/sync-agent-config` writes `CLAUDE.md`,
`AGENTS.md`, and `.claude/rules/`. Repo-owned skills live in `skills/`,
symlinked into `.claude/skills` and `.agents/skills` by `bin/link-skills`.
The Claude post-edit hook re-syncs automatically; the stop hook
(`bin/agent-stop-check`) runs the drift check plus `bin/check` before an
agent hands work back.

## Tooling reference

| Command                 | What it does                                                                  |
| ----------------------- | ----------------------------------------------------------------------------- |
| `bin/check`             | Full gate: fmt, shellcheck, build, vet, analyzers, golangci, tidy/codegen drift, tests |
| `bin/generate`          | Regenerate templ output, models (live schema), delegates (signatures)         |
| `bin/testdb`            | Recreate + migrate the test template database                                 |
| `bin/migration`         | Create a new tern migration                                                   |
| `bin/recreate`          | Recreate + migrate the dev database                                           |
| `bin/setup`             | Install the toolchain (mise) and generate `.env`                              |
| `bin/vuln-check`        | govulncheck — separate from `bin/check` because the vuln DB moves on its own  |
| `bin/coverage`          | Cross-package coverage with a first-party minimum                             |
| `bin/worktree-setup`    | Per-worktree Postgres clones + port allocation (writes `worktree.env`)        |
| `bin/worktree-teardown` | Drop the worktree's clones, free its port                                     |
| `bin/sync-agent-config` | Regenerate agent instruction files from `ai/`                                 |

CI (`.github/workflows/check.yml`) runs `bin/check` and `bin/vuln-check`
against a Postgres service on every push and PR — the same gate as the
stop hook, so nothing merges that an agent couldn't have handed back.

Tool versions are pinned in `.mise.toml`; mise also loads `.env` and
`worktree.env`. Custom analyzers live in `analyzers/` and run via
`go run ./cmd/lint ./...`. Stock linters via `.golangci.yml`.
Secrets/config come from `.env` (gitignored; start from `.env.example`).
