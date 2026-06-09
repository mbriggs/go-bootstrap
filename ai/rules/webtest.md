---
title: Real DB Tests
claudePaths:
  - "**/*_test.go"
---

Use the repo skill `webtest-workflow` at `skills/webtest-workflow/SKILL.md`
before adding or changing DB-backed tests, fixtures, or factories.

Tests hit a real per-package Postgres clone via
`func TestMain(m *testing.M) { webtest.Main(m) }`. Avoid mocking ordinary
query/command behavior or adding dependency-injection ceremony around domain
persistence. There is no per-test rollback — use unique fixture names and
reserve `t.Parallel()` for tests that only touch their own rows.
