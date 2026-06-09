---
title: HTTP Handler Workflow
claudePaths:
  - "web/**"
---

Use the repo skill `http-handler-workflow` at
`skills/http-handler-workflow/SKILL.md` before changing Echo handlers,
middleware, the render path, or session-bound auth.

Render server pages via `web.RenderPage` / `web.RenderPageData`
(buffer-first), shape JSON errors via `web/apierror`, manage flash via
`web.SetFlash` / `web.TakeFlash`, and branch on domain sentinels with
`errors.Is`. Keep domain behavior in domain packages, not in handlers.
