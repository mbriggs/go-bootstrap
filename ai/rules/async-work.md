---
title: Async Work
claudePaths:
  - "jobs/**"
  - "flows/**"
  - "mailer/**"
---

Background work has two tiers: `jobs/` (River) for single-step work
enqueued transactionally — `jobs.Client.InsertTx` in the same transaction
as the state change, so rollbacks can't orphan jobs; enqueues with no
accompanying state change make that claim by name via
`jobs.InsertStandalone` (the `jobconfine` analyzer confines River's plain
`Insert` to the jobs package) — and `flows/`
(Inngest) for durable multi-step orchestration. Workers and flow steps are
transport; behavior stays in domain packages. Email goes through the
`mailer` seam from workers or steps, never from request handlers.

Use the repo skill `async-work` at `skills/async-work/SKILL.md` before
adding any work that outlives a request — it carries the dividing line,
the enqueue rules, and the River migration upgrade procedure.
