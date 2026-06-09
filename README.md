# go-bootstrap

A batteries-included starting point for Go web services: Echo + pgx +
hand-written SQL migrations (tern), with the conventions from
[STANDARDS.md](STANDARDS.md) wired in as working tooling — schema-first
code generation, custom lint analyzers, and an integration test harness
with database-per-package isolation.

## Start a project

```sh
go run golang.org/x/tools/cmd/gonew@latest github.com/mbriggs/go-bootstrap github.com/you/yourproject
cd yourproject && git init

bin/setup        # installs toolchain, generates .env from the module name
```

Postgres: use a native install, or `docker compose up -d`. With native
Postgres, create the role and dev database once:

```sh
psql -d postgres -c "CREATE ROLE yourproject LOGIN PASSWORD 'yourproject' CREATEDB"
bin/recreate     # creates + migrates the dev database
```

Then verify the skeleton end to end:

```sh
bin/check        # fmt, build, vet, analyzers, golangci, codegen drift, tests
air              # dev server with hot reload
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

## What's inside

| Path           | What it is                                                       |
| -------------- | ---------------------------------------------------------------- |
| `db/`          | Generic data access: `Find[T]`, transactions, row-map helpers     |
| `logging/`     | Named per-package loggers with runtime filter DSL                 |
| `web/`         | Echo router skeleton + `apierror` JSON error responses            |
| `webtest/`     | Integration harness: template-DB-per-package isolation            |
| `testdata/`    | Concurrency-safe unique-name sequences for fixtures               |
| `cmd/modelgen` | Schema → model codegen (drift-checked, committed output)          |
| `cmd/txgen`    | `FooTx` → pool-delegate codegen                                   |
| `cmd/lint`     | Custom analyzers: `txparam`, `connconfine`                        |
| `bin/`         | `setup`, `check`, `generate`, `testdb`, `recreate`, `migration`   |
