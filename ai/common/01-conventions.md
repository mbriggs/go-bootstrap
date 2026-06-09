# Project conventions

Batteries-included Go web service template: Echo + pgx + tern migrations,
templ server rendering with scs sessions, schema-first code generation,
custom lint analyzers, and an integration test harness with
database-per-package isolation.

Read STANDARDS.md before writing code — it defines the conventions and
which ones are machine-enforced. docs/SYSTEM-MAP.md is the structural
map: boot sequence, request lifecycle, codegen flows, and which tool
enforces which invariant.

## Workflow

- `bin/check` must pass before any unit of work is done. It runs gofumpt,
  shellcheck, build, vet, the custom analyzers, golangci-lint, go.mod tidy
  drift, the codegen drift check, and the test suite. `bin/vuln-check` runs
  govulncheck separately because vulnerability database results can change
  without a code change.
- Schema changes: `bin/migration <name>` → write SQL → `tern migrate` →
  update `modelgen.yaml` if a new table → `bin/generate` → commit the
  generated output. Never hand-edit `*_gen.go`.
- Persistence: hand-write only `FooTx(ctx, tx db.Queryable, ...)` forms;
  `bin/generate` emits the bare delegates. Never reference `db.Conn` in
  hand-written code.
- Views: edit `.templ` sources under `views/`, then `bin/generate` (air does
  this in the dev loop). Never hand-edit `*_templ.go`. Compose pages from
  design-system components from `github.com/mbriggs/gesso/ui` — browse
  them at `/design`. The design system is a separate module pinned in
  go.mod; for local component work, check out gesso as a sibling and run
  `go work init . ../gesso` (go.work is gitignored). Component and style
  changes happen there, against gesso's own `bin/check`.
- Tests: every DB-touching package needs
  `func TestMain(m *testing.M) { webtest.Main(m) }`. Run `bin/testdb`
  once (and after migration changes) to build the template database.
  Tests touching only their own uniquely-named rows call `t.Parallel()`.

## Environment

- Process configuration goes through `env.Load()` once at startup —
  nothing else reads `os.Getenv` for app-level settings (PG* vars are the
  exception; pgx consumes those directly).
- PG connection comes from `.env`; `bin/setup` generates it from the module
  name and mise loads it (`worktree.env` overrides it in worktrees — see
  `bin/worktree-setup` for per-worktree DB and port isolation).
- Database names derive from the module path at runtime — renaming the
  module renames the databases.
- The dev server listens on `$PORT` (default 8080); `air` runs the hot
  rebuild loop.
