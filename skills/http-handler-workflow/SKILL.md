---
name: http-handler-workflow
description: Use when adding or changing Echo handlers or middleware under web/. Pick auth scope, render via templ or apierror JSON, manage flash, branch on domain errors, and keep handlers thin.
---

# HTTP Handler Workflow

## Shape

Routes and middleware are wired in `web/router.go`; handlers live in
`web/handlers_*.go` and have the Echo v5 signature
`func(c *echo.Context) error`. The shared stack is Echo's `RequestID`,
the hand-rolled request span (`web/tracing.go`), `Recover`, and request
logging, plus scs sessions (`web.Sessions.LoadAndSave` wrapped via
`echo.WrapMiddleware`) and `web.LoadUser`. There is no CORS layer —
sessions are same-origin; add `middleware.CORSWithConfig` with explicit
origins if the app grows cross-origin consumers. Gate signed-in groups
with `web.RequireUser`; gate role-bound groups with
`web.RequireRole("admin")`.

```go
g := e.Group("", web.RequireUser)
g.GET("/", web.Home)
```

## Render path

Server-rendered pages go through `web.RenderPage(c, meta, component)`:

- Renders into a buffer first so a mid-stream component failure produces a
  500, not a partial 200.
- Adds `User` and `Flash` to `views.LayoutData` automatically; the layout
  displays the flash, so handlers just `SetFlash` and re-render.
- Use `web.RenderPageData` only when the component itself needs the
  request-scoped `views.LayoutData`.

JSON endpoints render via `c.JSON` and shape errors with `web/apierror` —
a flat `{"message", "status"}` shape clients can parse and branch on.
`apierror.Internal` logs the detail and returns a generic 500; internals
never cross the boundary. If the service mimics an existing API, mimic
its error shape and status codes too.

Set flashes via `web.SetFlash(c, "ok" | "error", msg)`; the next render pops
and clears them via `web.TakeFlash`.

## Auth

- `web.CurrentUser(c)` returns the user attached by `LoadUser`, or nil.
- On signin success, call `web.Sessions.RenewToken(ctx)` before
  `Put("user_id", …)` so the session id rotates and fixation can't ride a
  pre-login cookie, and `Put("password_epoch", user.PasswordEpoch())` —
  `LoadUser` destroys any session whose epoch no longer matches, which is
  what signs every session out when the password changes.
- `auth.Authenticate` collapses unknown-email and bad-password into the same
  `auth.ErrInvalidCredentials` — don't distinguish them at the wire (it
  burns a decoy hash compare on unknown email so the two are
  timing-uniform too). Signin failures are throttled per (IP, email);
  record outcomes via the existing pattern in `SigninSubmit` if you add
  other credential checks. The layered defense: argon2id at OWASP params
  slows offline cracking (signin is rare, ~50ms is tolerable), the
  throttle slows online guessing. Throttle attempts live in Postgres
  (`throttle_attempts`), so the limit holds across processes;
  `web/throttle.go` owns the policy and the `*Tx` queries.
- Gate on arbitrary predicates with `web.RequirePolicy(policy)`; policies
  live next to the domain they protect and return an error explaining the
  denial (logged, never rendered).
- POST handlers that mutate state sit behind `web.RequireSameOriginPost`;
  POST→GET redirects funnel through `web.SafeRedirect` so redirect
  validation stays in one place.

## Domain errors at the seam

Handlers normally let errors propagate — the router's error handler
renders a templ error page for browsers (with full detail and a copy
button in development) and an `apierror` JSON body for API clients.
Branch inline when a domain error maps to a non-500 response:

```go
u, err := auth.Authenticate(ctx, email, password)
if errors.Is(err, auth.ErrInvalidCredentials) {
    web.SetFlash(c, "error", "invalid email or password")
    return web.RenderPage(c, web.PageMeta{Title: "Sign in"}, views.SigninPage(email))
}
```

Sentinel errors (`errors.Is`) are the discriminator; they live in the
package that owns the condition, and custom error types wait until
something must carry data a sentinel can't.

## Forms

- Call `web.ParseForm(c)` before reading form fields — it turns malformed
  bodies into a 400 instead of silently reading empty values.
- Standard mutations finish through `web.FinishMutation(c, web.FormResult{...})`
  (POST-redirect-GET): errors flash a safe `web.UserMessage` and bounce
  back, success flashes and redirects on. Use `ErrorFlashKey` to pin the
  message to a field.

## What stays out of handlers

- Domain logic, validation rules → the domain package.
- Multi-step DB work → `db.ExecInTx` inline is fine, but the body should
  call `FooTx` domain functions, not assemble SQL.
- Queries → handlers never import `db` for queries; they go through the
  domain package.

## Tests

Handler tests live in `web/` and run against the real per-package test
database (see the `webtest-workflow` skill). Session-bound flows use
`webtest.Server` + `webtest.NewClient`. Mock nothing.
