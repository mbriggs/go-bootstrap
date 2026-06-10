## Workflow

- `bin/check` must pass before any unit of work is done. It runs gofumpt,
  shellcheck, build, vet, the custom analyzers, golangci-lint, go.mod tidy
  drift, the codegen drift check, and the test suite. `bin/vuln-check` runs
  govulncheck separately because vulnerability database results can change
  without a code change.
- Schema changes: `bin/migration <name>` → write SQL → `bin/migrate up` →
  update `modelgen.yaml` if a new table → `bin/generate` → commit the
  generated output. Never hand-edit `*.gen.go`.
- Persistence: hand-write only `FooTx(ctx, tx db.Queryable, ...)` forms;
  `bin/generate` emits the direct `db.Conn` variants. Never reference
  `db.Conn` in hand-written code.
- Views: edit `.templ` sources under `views/`, then `bin/generate` (air does
  this in the dev loop). Never hand-edit `*_templ.go`. Compose pages from
  design-system components from `github.com/mbriggs/gesso/ui` — browse
  them at `/design`. The design system is a separate module pinned in
  go.mod; for local component work, check out gesso as a sibling and run
  `go work init . ../gesso` (go.work is gitignored). Component and style
  changes happen there, against gesso's own `bin/check`.
- Tests: every DB-touching package needs
  `func TestMain(m *testing.M) { webtest.Main(m) }`. Run `bin/testdb`
  after migration changes to rebuild the template database — `bin/check`
  verifies the template against the migrations (builds it only when
  missing) and fails if it's stale.
  Tests touching only their own uniquely-named rows call `t.Parallel()`.
- Async work: single-step jobs enqueue through `jobs.Client.InsertTx` in
  the same transaction as the state change (River); multi-step durable
  processes live in `flows/` (Inngest). Email sends through the `mailer`
  seam from workers or flow steps, never from request handlers.

## Logging

- One named logger per package: `var logger = logging.Logger("mypackage")`;
  runtime filtering via the settings DSL (`"_all,-db:debug"`). No leftover
  debugging — the leveled logger *is* the debug mechanism; if a line isn't
  worth keeping at `Debug` level, it isn't worth committing.
- When a ctx is in hand, log through the `*Context` variants
  (`logger.InfoContext(ctx, ...)`) — the handler stamps `trace_id`/`span_id`
  from the active span, so the line correlates with its trace. Plain calls
  still work; they just can't be correlated.

## Style
- `any`, not `interface{}`. Dot-imports only for `webtest` in test files.
  Match the file you're editing.

## Environment

- Process configuration goes through `env.Load()` once at startup —
  nothing else reads `os.Getenv` for app-level settings. SDK-consumed
  variable families are the exception: PG* (pgx), OTEL_* (OpenTelemetry;
  tracing turns on with `OTEL_EXPORTER_OTLP_ENDPOINT`), SENTRY_* (error
  tracking turns on with `SENTRY_DSN`; 5xx responses and discarded jobs
  report automatically), INNGEST_* (`INNGEST_DEV=1` in development).
- PG connection comes from `.env`; `bin/setup` generates it from the module
  name and mise loads it (`worktree.env` overrides it in worktrees — see
  `bin/worktree-setup` for per-worktree DB and port isolation).
- Database names derive from the module path at runtime — renaming the
  module renames the databases.
- The dev server listens on `$PORT` (default 8080); `air` runs the hot
  rebuild loop.
