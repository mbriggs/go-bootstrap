---
title: Server View Workflow
claudePaths:
  - "views/**"
  - "**/*.templ"
---

Use the repo skill `server-view-workflow` at
`skills/server-view-workflow/SKILL.md` before changing templ views, page
DTOs, or layout data.

Views render already-shaped data — no DB calls, sessions, or domain commands
inside `views/`. Edit `.templ` sources and regenerate via `bin/generate`;
never hand-edit `*_templ.go`.
