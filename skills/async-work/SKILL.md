---
name: async-work
description: Policy for background work — the jobs (River) vs flows (Inngest) split, transactional enqueue, and the mailer seam. Use BEFORE adding any work that outlives a request — sending email, calling an external service after a mutation, scheduling follow-ups, or building a multi-step process. The default instinct to do it inline in the handler, or to chain jobs into a state machine, is wrong here.
---

# Async work

## The two tiers

**jobs/ (River)** — single-step background work that follows from a
transaction. **flows/ (Inngest)** — durable multi-step processes that
coordinate over time.

Reach for jobs first. Reach for flows when you catch yourself building a
state machine out of chained jobs — encoding "what step comes next" in job
args, or persisting progress flags a worker checks.

| Signal | Tier |
| --- | --- |
| "This committed, so send the email / sync the search index" | jobs |
| Work must not exist if the request rolls back | jobs |
| "Do X now, check back in a day, abandon if they cancel" | flows |
| Waiting on an event or a human between steps | flows |
| Steps must survive deploys and retry independently | flows |

## Always

- Enqueue jobs inside the transaction that creates the state they follow
  from: `jobs.Client.InsertTx(ctx, tx, args, nil)`. The job becomes
  runnable only on commit, so a rollback can never leave an orphaned job.
  This transactional handoff is why River over an external broker —
  coupling to Postgres is a feature here. Work with no accompanying state
  change uses `jobs.InsertStandalone` — the name is the no-transaction
  claim, and the `jobconfine` analyzer confines River's plain `Insert`
  to the jobs package so the claim is always explicit. The reset request
  is the example: nothing mutates until the worker runs.
- Workers are transport. `Work` unpacks args and calls domain code;
  behavior lives in domain packages (`jobs/password_reset_email.go` is the
  shape: one domain call, one `mailer.Send`).
- Keep secrets out of job args — args sit in `river_job` until the
  cleaner retires the row. The reset worker mints its token at send time
  instead of receiving it; prefer "args identify, worker derives".
- Register workers in `jobs.Configure`, flows in `flows.Configure` —
  both before anything serves.
- Outbound email goes through the `mailer` seam (`mailer.Send`) from a
  worker or flow step. The default sender logs; production swaps
  `mailer.Outbox` at boot.
- In flows, side effects live inside `step.Run` so they're checkpointed
  and retried per-step; sleeps use `step.Sleep` (they survive deploys —
  Inngest re-invokes and replays recorded step results).
- Event names are `area/noun.action` (`app/user.created`); send with
  `flows.Send` from wherever the fact occurs.
- Tests assert on enqueued `river_job` rows (query `kind` + `args`).
  webtest configures the jobs client but never starts workers, so enqueued
  rows sit still for assertions.

## Never

- External IO (email, HTTP, shelling out) inline in a request handler
  after a mutation — the request then blocks on the provider and fails
  when it does. Enqueue instead.
- A bare `Insert` (non-Tx) for work tied to a mutation — if the mutation
  rolls back, the job still runs. Same silent-correctness class as a read
  escaping a transaction; `jobconfine` reports it, and reaching for
  `InsertStandalone` to silence it is making a false claim.
- State machines built from chained jobs. That's flows' job; River args
  carrying "current step" is the tell.
- Hand-editing the River goose migrations (`migrations/003`, `004`). They
  are dumped verbatim from River's migration line; on a River upgrade,
  regenerate with
  `go run github.com/riverqueue/river/cmd/river@<ver> migrate-get --version <new> --up`
  and add a new goose file.
- Starting job workers or mounting `/api/inngest` in webtest — tests
  observe enqueued work, they don't execute it.

**Why two tools:** each is kept at its strength. River alone means
hand-rolling sagas (retry bookkeeping, step state, timers in job args);
Inngest alone gives up transactional enqueue and puts an external service
on the request path for work that's really just "after commit, do one
thing". The dividing line keeps both honest, and the worked examples
(`jobs/password_reset_email.go`, `flows/welcome.go`) are the templates to
copy.
