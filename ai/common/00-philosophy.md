# Philosophy

Keep things simple, and care about ergonomics — everything else follows
from that. Go makes the instinct trustworthy: nothing hides under the
syntax, so when code feels elegant it actually is. The same honesty makes
ugliness a finely honed sensing tool — code turns ugly the moment it
takes on accidental complexity, and the response is to eliminate the
complexity, not to pad the code. The mess that remains is essential —
heavy IO is messy because IO is messy — and that gets isolated behind a
clean boundary so everything else stays clean. The bullets below are not
separate rules; they are that one judgment applied in different places.

- **Packages by default.** A package's exported functions are its
  interface; its file boundary is its encapsulation. Reach for a struct
  when it models a stateful process.
- **Files split by topic, never by kind.** No `types.go`, `helpers.go`,
  or `utils.go` — a file is named for the one responsibility it owns, and
  everything that responsibility needs lives together (`mailer/mailer.go`
  holds the seam's interface, the log-sender default, and the instance).
  The namesake file is the package's front door: the global, lifecycle,
  logger, shared sentinels. A one-topic package stays a single namesake
  file until a second topic earns the split; a struct that models a
  stateful process earns a file named for its role. Tests mirror their
  file, except flow tests, which are named for the behavior
  (`signin_test.go`). The only non-semantic name is the `.gen.go`
  suffix — provenance, marking generated-don't-touch.
- **Dependency injection marks architectural substitutability.** A seam
  is earned where substitution is part of the design: call sites that
  genuinely vary (pool vs. transaction), or an external service the app
  shouldn't be coupled to — databases are the deliberate exception;
  coupling to Postgres is a feature here. Never inject to make code
  abstractly "testable", but an earned seam pays testing back on its
  own: a log-backed implementation (`mailer.Outbox`) substitutes
  naturally — architecture solving what a mocking framework papers over.
- **Globals for thread-safe, single-instance resources** — the connection
  pool (`db.Conn`), the logger registry, the session manager
  (`web.Sessions`). There is exactly one of each per process; threading
  them through every signature is ergonomics lost with nothing gained.
  Anything that isn't both thread-safe and genuinely singular doesn't
  qualify.
- **Abstractions are earned.** A helper, generator, or interface is
  justified after the pattern it captures has been hand-written about
  three times — not before. Speculative infrastructure rots.

Conventions come in two strengths: **enforce what causes silent
correctness bugs; document what is an architectural judgment call.** A
convention whose violation produces a quiet wrong-answer (a read escaping
a transaction, a model drifting from the schema) gets machinery — an
analyzer, a generator, a drift check in `bin/check`. A convention whose
violation is visible erosion (a package in the wrong place, a boundary
crossed) gets documentation and review. If `bin/check` passes, the
enforced rules hold.

Deviations from mainstream Go are allowed when concretely justified, and
the justification lives in the skill that owns the convention.
