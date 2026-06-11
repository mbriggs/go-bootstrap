# go-bootstrap

A batteries-included starting point for Go web services: Echo + pgx +
hand-written SQL migrations (goose) + templ server rendering with scs
sessions, password auth (signin + reset), and the
[gesso](https://github.com/mbriggs/gesso) design system (browse it at
`/design`), with the project's conventions wired in as working tooling —
schema-first code generation, custom lint analyzers, an integration test
harness with database-per-package isolation, per-worktree environment
isolation, and generated agent configuration. Background jobs ride
[River](https://riverqueue.com) (transactional enqueue on the same
Postgres), durable multi-step orchestration rides
[Inngest](https://www.inngest.com) (`flows/`), OpenTelemetry tracing
lights up when `OTEL_EXPORTER_OTLP_ENDPOINT` is set, and Sentry error
tracking lights up when `SENTRY_DSN` is set (5xx responses, discarded
jobs, and job panics report automatically).

The bias throughout is simple, ergonomic Go: packages by default, structs
where there is state to model, essential mess isolated, accidental
complexity eliminated. [CLAUDE.md](CLAUDE.md) carries the philosophy;
each convention's reasoning lives in its skill under [skills/](skills/).
[docs/SYSTEM-MAP.md](docs/SYSTEM-MAP.md) is the structural map — boot
sequence, request lifecycle, codegen flows, and which tool enforces which
invariant.

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
go run ./cmd/createuser -email you@example.com -password a-secret-pw -roles admin
air              # dev server with hot reload (templ + go) on :8080
```

Everything is named after your module automatically — databases
(`yourproject_dev`, `yourproject_template`, per-package test clones), the
generated `.env` — so there is nothing to rename beyond what `gonew` does.

## Adding a model

```sh
bin/migration add_things_table   # write the SQL, then:
bin/migrate up
# add the table to modelgen.yaml:
#   - table: things
#     package: thing
#     type: Thing
bin/generate                     # model struct + ToRowMap from the schema
```

Hand-write only `FooTx(ctx, tx db.Queryable, ...)` persistence functions in
the new package; `bin/generate` emits the pool-backed bare forms. See the
[go-tx-pattern](skills/go-tx-pattern/SKILL.md) skill for the conventions
and the reasoning.

## Async work

Two tiers with one dividing line (the package docs carry it): `jobs/`
(River) for single-step background work — enqueued in the same
transaction as the state change it follows from, when there is one — and
`flows/` (Inngest) for multi-step processes that coordinate over time.
The password-reset email is the worked job example (worker as transport;
it mints the token at send time so the cleartext never persists). Reach
for jobs first; reach for flows when you catch yourself building a state
machine out of chained jobs. The flows dev server is
`docker compose up inngest` (UI at :8288, set `INNGEST_DEV=1`); the app
runs fine without it. `cmd/createuser` emits `app/user.created`
(best-effort), so with the dev server up, creating a user runs the
welcome flow end to end.

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
| `appname/`       | Module-derived process identity (database names, service name, app id) |
| `db/`            | Generic data access: `Find[T]`/`Insert[T]`/delete, transactions, row maps |
| `env/`           | Process configuration: read once, validated, passed as values     |
| `auth/`          | Users + argon2id credentials, enumeration-safe sentinels, throttled signin, password reset tokens (resets sign out live sessions) |
| `web/`           | Echo router, scs sessions, templ render path, flash, CSRF gate, security headers (nonce'd CSP), `apierror` |
| `jobs/`          | Background jobs (River): transactional enqueue, workers as transport  |
| `flows/`         | Durable orchestration (Inngest): checkpointed steps, sleeps, event waits |
| `mailer/`        | Outbound-email seam; dev sender logs, SES sends in production (`MAIL_FROM` + AWS_* env) |
| `telemetry/`     | OTLP tracing + Sentry error tracking, both env-gated; request spans (`web/tracing.go`) + otelpgx |
| `views/`         | templ layout + pages (`LayoutData`, signin, home, error)          |
| ([gesso](https://github.com/mbriggs/gesso)) | Design system dependency: templ components + embedded assets, browse at `/design` |
| `public/`        | Static assets served at `/public` (minimal `app.css`)             |
| `logging/`       | Named per-package loggers with runtime filter DSL                 |
| `webtest/`       | Integration harness: template-DB-per-package, wired server, cookie client |
| `partition/`     | Split batch parse results into values + failures                  |
| `fixture/`       | Test-data identity: deterministic fixture ids, unique-name sequences |
| `cmd/modelgen`   | Schema → model codegen (drift-checked, committed output)          |
| `cmd/conngen`    | `FooTx` → direct `db.Conn` variant codegen                        |
| `cmd/createuser` | Provision the first login from the CLI                            |
| `cmd/migrate`    | goose migrations against the PG* environment (`bin/migrate`)      |
| `cmd/lint`       | Custom analyzers: `txparam`, `connconfine`, `jobconfine`          |
| `ai/`, `skills/` | Sources for generated agent config + repo-owned skills            |
| `bin/`           | check, generate, testdb, recreate, migration, setup, coverage, vuln-check, worktree + port tools, agent-config sync |
