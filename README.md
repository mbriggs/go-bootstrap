# go-bootstrap

A batteries-included starting point for Go web services: Echo + pgx +
hand-written SQL migrations (tern) + templ server rendering with scs
sessions, password auth, and the [gesso](https://github.com/mbriggs/gesso)
design system (browse it at `/design`), with the conventions from
[STANDARDS.md](STANDARDS.md) wired in as working tooling — schema-first
code generation, custom lint analyzers, an integration test harness with
database-per-package isolation, per-worktree environment isolation, and
generated agent configuration.

[docs/SYSTEM-MAP.md](docs/SYSTEM-MAP.md) is the structural map — boot
sequence, request lifecycle, codegen flows, and which tool enforces which
invariant. [STANDARDS.md](STANDARDS.md) is the reasoning behind them.

## Start a project

```sh
go run golang.org/x/tools/cmd/gonew@latest github.com/mbriggs/go-bootstrap github.com/you/yourproject
cd yourproject && git init

bin/setup        # installs the toolchain via mise, generates .env from the module name
```

Postgres: use a native install, or `docker compose up -d`. With native
Postgres, create the role and dev database once:

```sh
psql -d postgres -c "CREATE ROLE yourproject LOGIN PASSWORD 'yourproject' CREATEDB"
bin/recreate     # creates + migrates the dev database
```

Then verify the skeleton end to end and create the first login:

```sh
bin/check        # fmt, shellcheck, build, vet, analyzers, golangci, drift, tests
go run ./cmd/createuser -email you@example.com -password secret -roles admin
air              # dev server with hot reload (templ + go) on :8080
```

Everything is named after your module automatically — databases
(`yourproject_dev`, `yourproject_template`, per-package test clones), the
generated `.env` — so there is nothing to rename beyond what `gonew` does.

## Adding a model

```sh
bin/migration add_things_table   # write the SQL, then:
tern migrate
# add the table to modelgen.yaml:
#   - table: things
#     package: thing
#     type: Thing
bin/generate                     # model struct + ToRowMap from the schema
```

Hand-write only `FooTx(ctx, tx db.Queryable, ...)` persistence functions in
the new package; `bin/generate` emits the pool-backed bare forms. See
[STANDARDS.md](STANDARDS.md) for the conventions and the reasoning.

## Worktrees

Each git worktree gets its own Postgres clones and dev-server port:
`bin/worktree-setup` clones the dev DB and test template, allocates a port,
and writes `worktree.env` (loaded by mise) so `air`, `bin/testdb`, and the
tests all land on the worktree's copies. `.config/wt.toml` wires this into
[worktrunk](https://worktrunk.dev); it works with plain `git worktree add`
too. `bin/worktree-teardown` drops the clones and frees the port.

## Agent configuration

`CLAUDE.md`, `AGENTS.md`, and `.claude/rules/` are generated — edit
`ai/common/` and `ai/rules/`, then run `bin/sync-agent-config`. Repo-owned
skills live in `skills/`. The Claude stop hook runs the drift check plus
`bin/check` before an agent hands work back.

## What's inside

| Path             | What it is                                                       |
| ---------------- | ---------------------------------------------------------------- |
| `db/`            | Generic data access: `Find[T]`/`Insert[T]`/delete, transactions, row maps |
| `env/`           | Process configuration: read once, validated, passed as values     |
| `auth/`          | Users + bcrypt credentials, enumeration-safe sentinels, throttled signin |
| `web/`           | Echo router, scs sessions, templ render path, flash, CSRF gate, `apierror` |
| `views/`         | templ layout + pages (`LayoutData`, signin, home, error)          |
| ([gesso](https://github.com/mbriggs/gesso)) | Design system dependency: templ components + embedded assets, browse at `/design` |
| `public/`        | Static assets served at `/public` (minimal `app.css`)             |
| `logging/`       | Named per-package loggers with runtime filter DSL                 |
| `webtest/`       | Integration harness: template-DB-per-package, wired server, cookie client |
| `partition/`     | Split batch parse results into values + failures                  |
| `fixtureid/`     | Deterministic fixture ids clear of serial-pk ranges               |
| `testdata/`      | Concurrency-safe unique-name sequences for fixtures               |
| `cmd/modelgen`   | Schema → model codegen (drift-checked, committed output)          |
| `cmd/txgen`      | `FooTx` → pool-delegate codegen                                   |
| `cmd/createuser` | Provision the first login from the CLI                            |
| `cmd/lint`       | Custom analyzers: `txparam`, `connconfine`                        |
| `ai/`, `skills/` | Sources for generated agent config + repo-owned skills            |
| `bin/`           | check, generate, testdb, recreate, migration, setup, coverage, vuln-check, worktree + port tools, agent-config sync |
