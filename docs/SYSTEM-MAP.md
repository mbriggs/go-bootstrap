# System map

Where things live, how a request flows, and which tool enforces which
invariant. [CLAUDE.md](../CLAUDE.md) carries the philosophy; each
convention's policy and reasoning lives in its skill under
[skills/](../skills/). This page is the *where*.

## Boot

`server.go` `run()`, in order:

1. `env.Load()` — `APP_ENV`/`PORT`/`PUBLIC_DIR`/`APP_URL` into an
   `env.Env` value; bad values fail here, not at first use.
2. `logging.Configure` — named per-package slog loggers with a runtime
   filter DSL (`logging/`).
3. `telemetry.Configure` — OTLP tracing when
   `OTEL_EXPORTER_OTLP_ENDPOINT` is set, the global no-op provider
   otherwise; shutdown flushes spans on exit.
4. `db.Configure` — opens the `pgxpool` (with the otelpgx query tracer)
   and sets the `db.Conn` global (the only place outside generated code
   allowed to touch it; the `connconfine` analyzer enforces that).
5. `web.Configure(db.Conn, cfg.AppEnv)` — builds `web.Sessions`
   (scs + pgxstore), keeps the pool for `/ready`, and latches dev/prod
   mode for error detail and the design gallery.
6. `jobs.Configure(db.Conn, cfg.BaseURL)` + `jobs.Start` — the River
   client and its workers (webtest configures without starting).
7. `web.Router(ctx, cfg.PublicDir)` — panics if Configure was skipped.
8. `flows.Configure()` — the Inngest client and durable functions; main
   mounts the returned handler at `/api/inngest`.
9. `http.Server` with full timeouts (header/read/write/idle); binds
   `localhost:PORT` in dev, `:PORT` otherwise; SIGTERM/Ctrl-C drains HTTP
   for up to 15s, then drains job workers.

## Request lifecycle

Middleware order in `web/router.go`:

```
CORS → RequestID → otelecho → Recover → request logger (logs request id)
  → scs LoadAndSave            (sessions; flash + signin state)
  → SameOriginPost             (web/secure.go: Origin / Sec-Fetch-Site gate,
                                X-Forwarded-Proto trusted for scheme,
                                /api/inngest exempt — signature-authed, no cookies)
  → LoadUser                   (web/auth.go: session user id → auth.User,
                                vanished users clear the session)
  → route handler              (RequireUser / RequirePolicy per route)
```

Client IPs come from the socket (`echo.ExtractIPDirect`) so spoofed
`X-Forwarded-For` can't rotate throttle keys; behind a reverse proxy,
swap the extractor for one with the proxy's trusted range. `/health` is
liveness; `/ready` pings Postgres for readiness.

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
modelgen.yaml entry  →  bin/generate  →  foo/<table>.gen.go   (struct + ToRowMap)
hand-write FooTx(ctx, tx db.Queryable, ...)  →  bin/generate  →  foo/conn.gen.go
```

Both generators are syntax/schema-driven — conngen never type-checks, so
generation works even while callers of the about-to-be-generated bare
forms don't compile yet.

Runtime rules (enforced by `cmd/lint` analyzers, not convention):

- `db.Conn` appears only in generated `conn.gen.go` files, `db/`, `main`,
  and `webtest` (`connconfine`).
- Hand-written persistence takes an explicit `tx db.Queryable` param
  (`txparam`); callers use bare forms or open `db.InTx`/`db.ExecInTx`.
- Generic helpers in `db/`: `FindTx`/`FindAllTx` (`query.go`),
  `InsertTx`/`UpdateTx`/`DeleteTx` (`mutate.go`) over `mbriggs/pgsql`
  builders.

## Async work

Two tiers, two packages, one dividing line (spelled out in `flows/flows.go`):

- `jobs/` — River, Postgres-backed. Single-step background work. Work
  tied to a mutation enqueues with `jobs.Client.InsertTx(ctx, tx, args, nil)`
  in the same tx, so a rollback never leaves an orphaned job; work with no
  accompanying state change uses plain `Insert`. Workers are transport —
  they unpack args and call domain code. The password-reset email is the
  worked example (`jobs/password_reset_email.go`): args carry only the
  email, the worker mints the token at send time.
- `flows/` — Inngest. Durable multi-step orchestration: checkpointed
  steps, sleeps that survive deploys, event waits. Served at
  `/api/inngest`; dev server via `docker compose up inngest`
  (UI at :8288, set `INNGEST_DEV=1`). The welcome drip is the worked
  example (`flows/welcome.go`), triggered by `app/user.created`, which
  `cmd/createuser` emits — so the tier demos end to end in dev.
- `mailer/` — the outbound-email seam both use; the default sender logs,
  production swaps `mailer.Outbox` at boot.

River's schema rides goose (`migrations/003`/`004`, dumped from River's
migration line — regenerate on River upgrades, don't hand-edit).

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
| Tests against real Postgres                | `bin/check` → `bin/testdb --check` + `go test -race ./...` |
| Known-vuln dependencies                    | `bin/vuln-check` (`govulncheck`, separate because the DB moves on its own) |
| Agent docs match `ai/` sources             | `bin/sync-agent-config --check` (stop hook)  |
| All of the above on every agent stop       | `bin/agent-stop-check`                       |
| All of the above on every push             | `.github/workflows/check.yml`                |

## Tooling reference

| Command                 | What it does                                                                  |
| ----------------------- | ----------------------------------------------------------------------------- |
| `bin/check`             | Full gate: fmt, shellcheck, build, vet, analyzers, golangci, tidy/codegen drift, tests |
| `bin/generate`          | Regenerate templ output, models (live schema), conn variants (signatures)     |
| `bin/testdb`            | Rebuild the test template (scratch-build + swap); `--check` verifies, building only if missing |
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
