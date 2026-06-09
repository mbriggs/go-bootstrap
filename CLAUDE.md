# CLAUDE.md

Read STANDARDS.md before writing code — it defines the conventions and
which ones are machine-enforced.

## Workflow

- `bin/check` must pass before any unit of work is done. It runs fmt,
  build, vet, the custom analyzers, golangci-lint, the codegen drift
  check, and the test suite.
- Schema changes: `bin/migration <name>` → write SQL → `tern migrate` →
  update `modelgen.yaml` if a new table → `bin/generate` → commit the
  generated output. Never hand-edit `*_gen.go`.
- Persistence: hand-write only `FooTx(ctx, tx db.Queryable, ...)` forms;
  `bin/generate` emits the bare delegates. Never reference `db.Conn` in
  hand-written code.
- Tests: every DB-touching package needs
  `func TestMain(m *testing.M) { webtest.Main(m) }`. Run `bin/testdb`
  once (and after migration changes) to build the template database.
  Tests touching only their own uniquely-named rows call `t.Parallel()`.

## Environment

- PG connection comes from `.env` (direnv); `bin/setup` generates it
  from the module name.
- Database names derive from the module path at runtime — renaming the
  module renames the databases.
