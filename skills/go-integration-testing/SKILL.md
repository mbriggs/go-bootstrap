---
name: go-integration-testing
description: Policy for Go tests — integration-first against real Postgres; mocks only for external services. Use BEFORE writing any Go test that touches a database, BEFORE introducing a mock/fake/stub, and BEFORE defining an interface whose purpose is testability. The default instinct to mock the data layer is wrong here.
---

# Go integration testing

## Core rule

Test through public interfaces against real infrastructure. The HTTP
handler (or the package's exported API) is the primary entry point; the
database is real Postgres, never mocked.

## The substitution bar

A substitute (mock/fake) must **pay rent in more than one context** —
e.g. a fake third-party API that is also useful for local development.
A mock that exists only so a test can run fails the bar. Corollary:
never define an interface whose only consumer is a test. Internal seams
(like `db.Queryable`) exist for runtime composition, not mocking.

Mocks are therefore reserved for **external services** (third-party
APIs). External services are also where DI is earned — the app shouldn't
be coupled to them (databases are the deliberate exception) — so the
fake is an in-memory implementation behind that seam: a real, runnable
component, substitution through architecture rather than a mocking
framework.

## Isolation: database-per-package

- Every DB-touching test package declares:

  ```go
  func TestMain(m *testing.M) { webtest.Main(m) }
  ```

  which clones a fresh database from a migrated **template database**
  (`CREATE DATABASE x TEMPLATE tpl` — file-copy fast), points the pool
  at it, and drops it after. `go test ./...` runs packages as separate
  processes, so packages parallelize freely, one global pool each.
- The template is rebuilt by script (`bin/testdb`) after migrations
  change — tests fail fast with a clear message if it's missing.
- Never run tests against the dev database, and never rely on
  accumulated rows.

## Within a package: the t.Parallel contract

- A test that touches only rows it created — via unique fixture names
  from a concurrency-safe test-data sequence — calls `t.Parallel()`.
- A test that asserts on table-wide state (counts, list endpoints)
  stays serial. That's the entire contract.
- No per-test transaction rollback and no truncation: rollback requires
  ambient transactions (rejected), truncation wipes parallel neighbors.

## Always

- Drive the behavior through the public path (request → handler → db),
  then assert through reads — not by inspecting internals.
- Test-first for behavior changes; one behavior per test; vertical
  slices (test → impl → repeat), never all-tests-then-all-code.
- Unit tests are for pure logic that is awkward to reach through the
  public path — and they live next to it.

## Never

- Mock the repository/data layer.
- Assert on implementation details (SQL strings, call counts).
- Heavy work in package `init()` — setup belongs in `TestMain`, where
  cleanup is possible.

**Why:** the bugs worth catching live in the seams (handler↔model↔SQL↔
schema). Mock-heavy suites pass while those seams are broken; this
suite deadlocked the moment a connection leak existed — that's the
sensitivity integration-first buys.
