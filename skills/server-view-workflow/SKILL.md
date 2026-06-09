---
name: server-view-workflow
description: Use when adding or changing templ views, page DTOs, layout data, or server-rendered UI under views/.
---

# Server View Workflow

## Boundary

Handlers talk to the domain. Views render already-shaped data.

- Keep DB calls, `context.Context`, `net/http`, sessions, and domain
  commands out of `views/`.
- Keep domain-to-page translation in the handler when it is tiny, or in a
  `web/*_pages.go` assembler when it grows. These functions return
  `views.*Page` DTOs.
- `views/` owns `.templ` files, generated `*_templ.go`, render DTOs,
  `views.LayoutData`, `views.Flash`, and small display helpers.

## Page Data Shape

Build DTOs around what the UI renders:

- labels and formatted strings instead of raw domain values when possible
- booleans for visibility and state
- rows/tiles in render order
- permission flags from handler policy checks

If a template needs to ask a domain question, move that decision into the
handler or assembler and pass the answer in the DTO.

## Components

Before inventing markup, look at:

- the gesso design system (`github.com/mbriggs/gesso/ui`) — components
  (buttons, forms, fields, alerts, badges, menus, modals, comboboxes,
  tables, surfaces) and `classes.go` for tone/size class helpers.
- `/design` in the running app — gesso's gallery renders every component
  and state (hidden in production).
- Nearby `views/*.templ` files for page-local patterns.

Prefer `ui` components for anything they cover. Page-specific markup
stays in the page template; components worth sharing across apps go into
gesso (sibling checkout + `go work init . ../gesso`, run gesso's
`bin/check` — it includes a dead-CSS linter, so a new component's demo
belongs in `gallery/` alongside it; then tag and bump the pin here).

## Rendering

- Handlers render through `web.RenderPage(c, meta, component)`; it buffers
  before writing — do not bypass it for full pages.
- Use `web.RenderPageData` only when the component itself needs the
  request-scoped `views.LayoutData`; the layout already displays flash and
  the signed-in user.
- Do not hand-edit generated `*_templ.go` files. Edit the `.templ` source,
  then run `bin/generate` (air does this automatically in the dev loop).

## Verification

After view or DTO changes:

```sh
bin/generate
go test ./views/... ./web/...
```

`bin/check` fails on uncommitted `*_templ.go` drift, so commit regenerated
output with the `.templ` change.
