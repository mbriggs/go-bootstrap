---
title: Agent Config Is Generated
claudePaths:
  - "ai/**/*.md"
  - "**/CLAUDE.md"
---

Use the repo skill `agent-config` at `skills/agent-config/SKILL.md` before
changing these files.

Do not edit generated instruction files directly. Edit `ai/common/*.md`,
`ai/rules/*.md`, or nested `CLAUDE.md` files, then run `bin/sync-agent-config`.
Use `bin/sync-agent-config --check` to fail on drift without writing.
