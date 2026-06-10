# System map

Where things live, how a request flows, and which tool enforces which
invariant. [CLAUDE.md](../CLAUDE.md) carries the philosophy; each
convention's policy and reasoning lives in its skill under
[skills/](../skills/). This page is the *where*.

## Boot

`server.go` `run()`, in order:

1. `env.Load()` — `APP_ENV`/`PORT`/`PUBLIC_DIR` into an `env.Env` value;
   bad values fail here, not at first use.
2. `logging.Configure` — named per-package slog loggers with a runtime
   filter DSL (`logging/`).
3. `db.Configure` — opens the `pgxpool` and sets the `db.Conn` global
   (the only place outside generated code allowed to touch it; the
   `connconfine` analyzer enforces that).
4. `web.Configure(db.Conn, cfg.AppEnv)` — builds `web.Sessions`
   (scs + pgxstore) and latches dev/prod mode for error detail and the
   design gallery.
5. `web.Router(ctx, cfg.PublicDir)` — panics if Configure was skipped.
6. `http.Server` with `ReadHeaderTimeout`; binds `localhost:PORT` in dev,
   `:PORT` otherwise; SIGTERM/Ctrl-C drains for up to 15s.

## Request lifecycle

Middleware order in `web/router.go`:

```
CORS → RequestID → Recover → request logger
  → scs LoadAndSave            (sessions; flash + signin state)
  → SameOriginPost             (web/secure.go: Origin / Sec-Fetch-Site gate,
                                X-Forwarded-Proto trusted for scheme)
  → LoadUser                   (web/auth.go: session user id → auth.User,
                                vanished users clear the session)
  → route handler              (RequireUser / RequirePolicy per route)
```

Handlers return errors; `errorHandler` (`web/errors.go`) is the one place
that shapes them — templ error page for HTML clients, `apierror` JSON
otherwise, stack detail (and a copy button) only in dev, 5xx logged.

Full pages render through `web.RenderPage` (`web/render.go`): handler
builds a `views.*Page` DTO, the layout (`views/layout.templ`) adds flash,
signed-in chrome, and asset links. Form posts follow
POST-redirect-GET via `web.FinishMutation` (`web/forms.go`); flash
messages ride the session (`web/flash.go`).

## Persistence

One model's lifecycle:

```
bin/migration foo  →  migrations/<version>_foo.sql  →  bin/migrate up
modelgen.yaml entry  →  bin/generate  →  foo/model.gen.go   (struct + ToRowMap)
hand-write FooTx(ctx, tx db.Queryable, ...)  →  bin/generate  →  foo/conn.gen.go
```

Runtime rules (enforced by `cmd/lint` analyzers, not convention):

- `db.Conn` appears only in generated `conn.gen.go` files, `db/`, `main`,
  and `webtest` (`connconfine`).
- Hand-written persistence takes an explicit `tx db.Queryable` param
  (`txparam`); callers use bare forms or open `db.InTx`/`db.ExecInTx`.
- Generic helpers in `db/`: `FindTx`/`FindAllTx` (`query.go`),
  `InsertTx`/`UpdateTx`/`DeleteTx` (`mutate.go`) over `mbriggs/pgsql`
  builders.

## Frontend assets

- The design system is the [gesso](https://github.com/mbriggs/gesso)
  module (`import "github.com/mbriggs/gesso/ui"`): templ components,
  tone/size class helpers, and embedded assets served at `/ui/*` via
  `ui.Assets()`. The version is pinned in go.mod; for local component
  work, check out gesso as a sibling and `go work init . ../gesso`
  (go.work is gitignored). Component and style work happens there, gated
  by gesso's own `bin/check` (including its dead-CSS linter).
- Asset URLs are content-hashed: `ui.AssetPath("ui.css")` →
  `/ui/<hash>/ui.css`, served `immutable`; the hash covers every embedded
  asset, so a dependency bump that changes any of them gets fresh URLs.
- `ui.css` is the style entry (`@layer theme, base, components, app`);
  `ui.js` is the behavior entry — an ES module importing one file per
  component contract by bare `ui/*` specifier, resolved by
  `ui.ImportMap()` in the layout head.
- `public/app.css` is the app's own `@layer app` chrome, served at
  `/public` — replace freely; the design system doesn't depend on it.
- `/design` renders gesso's gallery — every component and state with a
  scrollspy nav (`web/handlers_design.go` wrapping `gallery.Page`);
  404 in production.

## Tests

- `webtest.Main(m)` in a package's `TestMain` clones
  `<project>_template` → `<project>_test_<pid>`: database per package, no
  per-test rollback, uniqueness (factories, `fixture.NewSequence`,
  `fixture.ID`) is what makes `t.Parallel()` safe.
- `webtest.Server(ctx)` is the production-wired Echo app;
  `webtest.NewClient` carries cookies so signin flows test end to end.
- `auth.Make(ctx, opts...)` is the factory convention; new aggregates
  grow a `Make` of the same shape.
- `bin/testdb` rebuilds the template after migration changes;
  `worktree.env` (`bin/worktree-setup`) points a worktree at its own
  clones via `TEMPLATE_DB`/`PGDATABASE`/`PORT`.

## Enforcement map

| Invariant                                  | Enforced by                                  |
| ------------------------------------------ | -------------------------------------------- |
| Formatting, shell hygiene                  | `bin/check`: `gofumpt -l`, `shellcheck`      |
| go.mod tidy                                | `bin/check` tidy-drift step                  |
| `db.Conn` confinement, explicit tx params  | `cmd/lint` (`connconfine`, `txparam`) via golangci |
| Generated code matches sources             | `bin/check` drift gate on `*.gen.go` / `*_templ.go` (regenerate + stage) |
| Lint policy                                | `.golangci.yml`                              |
| No unreachable design-system CSS           | gesso's `cmd/cssdead` via gesso's `bin/check` |
| Tests against real Postgres                | `bin/check` → `bin/testdb` + `go test ./...` |
| Known-vuln dependencies                    | `bin/vuln-check` (`govulncheck`, separate because the DB moves on its own) |
| Agent docs match `ai/` sources             | `bin/sync-agent-config --check` (stop hook)  |
| All of the above on every agent stop       | `bin/agent-stop-check`                       |
| All of the above on every push             | `.github/workflows/check.yml`                |

## Tooling reference

| Command                 | What it does                                                                  |
| ----------------------- | ----------------------------------------------------------------------------- |
| `bin/check`             | Full gate: fmt, shellcheck, build, vet, analyzers, golangci, tidy/codegen drift, tests |
| `bin/generate`          | Regenerate templ output, models (live schema), conn variants (signatures)     |
| `bin/testdb`            | Recreate + migrate the test template database                                 |
| `bin/migration`         | Create a new goose migration                                                  |
| `bin/migrate`           | Run migrations (`up`, `down`, `status`, … via cmd/migrate)                    |
| `bin/recreate`          | Recreate + migrate the dev database                                           |
| `bin/setup`             | Install the toolchain (mise) and generate `.env`                              |
| `bin/vuln-check`        | govulncheck — separate from `bin/check` because the vuln DB moves on its own  |
| `bin/coverage`          | Cross-package coverage with a first-party minimum                             |
| `bin/worktree-setup`    | Per-worktree Postgres clones + port allocation (writes `worktree.env`)        |
| `bin/worktree-teardown` | Drop the worktree's clones, free its port                                     |
| `bin/sync-agent-config` | Regenerate agent instruction files from `ai/`                                 |

Tool versions are pinned in `.mise.toml`; mise also loads `.env`
(gitignored; start from `.env.example`) and `worktree.env`. Custom
analyzers live in `analyzers/` and run via `go run ./cmd/lint ./...`.

## Directory map

`README.md` has the table of packages. Start-here files: `server.go`
(boot), `web/router.go` (routes + middleware), `db/db.go` (pool + tx),
`auth/users.go` (domain example), `webtest/webtest.go` (test harness),
`../gesso/ui/components.go` (design-system surface), `env/env.go` (config).
