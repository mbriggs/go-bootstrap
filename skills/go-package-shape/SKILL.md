---
name: go-package-shape
description: Policy for Go program structure — packages by default; a struct earns its place by modelling a stateful process. Use BEFORE creating a new Go package, adding a new file to a package (files split by topic, never by kind — no types.go/helpers.go/utils.go), adding any service/repository/manager/handler struct whose fields are dependencies, creating an internal/ directory, or threading a dependency through constructors. Default Go-community habits (constructor-injected stateless service structs, internal/ trees, DI containers) are the wrong shape here.
---

# Go package shape

## Core rule

The package is the default unit of organization. Its exported functions
are the interface; its unexported identifiers are the encapsulation. A
struct earns its place by modelling something: data, or a stateful
process. The test is per-instance state — would two instances ever
differ? A stateful process may well carry its dependencies as fields;
what a struct can't be is a stateless module in disguise.

## Always

- **One package per domain aggregate** — a thing with its own table and
  lifecycle (`docindex`, `apikey`). The package owns its model and its
  persistence functions.
- **A bounded context is a directory; its subdirectories are its
  aggregates** (`billing/invoice`, `billing/payment`). The tree shows
  membership — the import path carries the context — and documentation
  carries the meaning: `docs/CONTEXT-MAP.md` describes each context and
  what it owns, as contexts appear. Crossing a context boundary is a
  review conversation; boundaries are reviewed, not compiler-enforced.
- **The repo root stays shallow.** Mechanism and platform packages at
  the top level (`db`, `web`, `logging`); extra binaries under `cmd/`;
  nothing nests deeper than context/aggregate. A lone aggregate may sit
  at the root until a second one earns the context directory. No
  `internal/`.
- **Split mechanism from domain.** Generic machinery (query helpers,
  transaction boundaries) lives in its own package (`db`); domain
  packages own their specific SQL. Handlers call domain packages, never
  the mechanism package directly (sentinel errors excepted). This is the
  general move: eliminate accidental complexity; isolate the essential
  mess that remains so everything else stays clean.
- **Globals are allowed for thread-safe, single-instance resources
  only** — the connection pool, the logger registry. Access is confined
  (lint where available): bootstrap, generated code, `main`.

## Never

- A stateless struct whose fields are process-wide singletons and whose
  methods are the real API (`type UserService struct { db *DB; logger
  *Logger }`). Two instances would never differ — there is nothing for
  the struct to model. Make it a package.
- Constructor injection for things that are not architecturally
  substitutable. DI is earned where substitution is part of the design:
  call sites that vary (pool vs. transaction), or an external service
  the app shouldn't be coupled to (databases are the deliberate
  exception). Never "for testability" — an earned seam already covers
  testing: an in-memory implementation substitutes through architecture,
  no mocking framework needed.
- A package per struct. A projection or value type lives with the
  aggregate or handler that shapes it.
- An orchestration/“service layer” package before an operation genuinely
  belongs to neither aggregate. Cross-aggregate logic defaults to the
  aggregate that owns the invariant.

## Files within a package

Files split by topic, never by kind. A file is named for the one
responsibility it owns; the test is whether you could predict the
filename from a one-sentence description of the code. No `types.go`,
`helpers.go`, or `utils.go` — a kind-bucket has infinite gravitational
pull, and cohesion dies there.

- **The namesake file is the package's front door**: the global, the
  lifecycle (`Configure`/`Close`), the package logger, shared sentinels.
  A one-topic package stays a single namesake file until a second topic
  earns the split.
- **Co-locate a seam's whole story.** The interface, its default
  implementation, and the package-level instance live in one file
  (`mailer/mailer.go`) — the reader gets all of it in one screen, and the
  seam is still there when a second implementation shows up.
- **A stateful-process struct gets its own file**, named for the role
  (`logging/logger_manager.go`, `webtest/client.go`).
- **Kind prefixes that subdivide by topic are fine; kind buckets are
  not.** `handlers_auth.go` / `handlers_design.go` sort the HTTP surface
  together while still splitting by feature — that's deliberate.
  `handlers.go` would be the bucket the rule bans.
- **Tests mirror their file** (`mutate_test.go`); flow tests are named
  for the behavior they exercise (`signin_test.go`), not a source file.
- **`.gen.go` is the only provenance-based name.** Every other filename
  says what the code is about, never where it came from.

## Right-sizing

"One topic" has no size cap — ugliness is the sensor. When a file holds
more than one thing you'd name in a sentence (validation grown into its
own subsystem, error mapping with real logic of its own), the topic has
become two; split the file along the seam that appeared.

File-by-topic deliberately provides no second level of structure — no
subdirectories within a package. When topics multiply past easy scanning,
or a cluster of files mostly talks to itself, that cluster is an
aggregate trying to leave: split the package — the new aggregate lands
as a sibling in the context directory — never add a subdirectory inside
a package.

The two conventions are load-bearing for each other. File-by-topic works
because packages stay aggregate-sized; packages stay aggregate-sized
because the only pressure-release valve is a package split. Weakening
either one breaks both.

## Shape

```go
// Wrong: stateless struct-as-bucket — two instances would never differ
type IndexService struct {
    db     *pgxpool.Pool
    logger *slog.Logger
}
func (s *IndexService) ByName(...) {...}

// Right: package with confined singletons
package docindex

var logger = logging.Logger("docindex")

func IndexByNameTx(ctx context.Context, tx db.Queryable, name string) (Index, error) {...}

// Also right: a struct, because it models a stateful process — and it
// carries the dependencies that process needs
type Import struct {
    tx   db.Queryable       // this run's transaction
    seen map[string]bool    // this run's progress
}
```

**Why:** there is exactly one app, one pool, one logger registry —
threading them through every call adds ceremony without information,
and the stateless-struct form invites mock-driven tests and DI sprawl.
When there is real per-instance state, the struct is modelling
something, and the same fields stop being ceremony.
