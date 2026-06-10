---
name: go-tx-pattern
description: Policy for Go database access — the Foo/FooTx pairing and pool confinement. Use BEFORE writing any Go function that queries or mutates a database (pgx or otherwise), opening a transaction, or referencing a connection-pool global. Violations are silent correctness bugs (reads escaping transactions), not style nits.
---

# Go transaction pattern

## Core rule

Hand-write persistence functions ONLY in their transaction-composable
form: `FooTx(ctx context.Context, tx db.Queryable, ...)`. The bare form
(`Foo`) is a generated direct `db.Conn` variant — never hand-written.

```go
// Hand-written: the only place logic lives
func IndexByNameTx(ctx context.Context, tx db.Queryable, name string) (Index, error) {
    return db.FindTx[Index](ctx, tx, `SELECT * FROM indexes WHERE name = $1`, name)
}

// Generated (conngen) — do not write this by hand:
func IndexByName(ctx context.Context, name string) (Index, error) {
    return IndexByNameTx(ctx, db.Conn, name)
}
```

`db.Queryable` is the seam (satisfied by both `*pgxpool.Pool` and
`pgx.Tx`):

```go
type Queryable interface {
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
    Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}
```

## Always

- Inside `db.InTx`/`db.ExecInTx`, every persistence call is the `*Tx`
  form passing the transaction. Calling a bare form inside a transaction
  is the bug this pattern exists to prevent — the read silently runs on
  the pool and cannot see the transaction's writes.
- Use the `tx` parameter you accepted. An ignored `Queryable` parameter
  means the body queries through something else (analyzer: `txparam`).
- Keep the pool global confined: only generated `conn.gen.go` files, the db
  package's bootstrap (`db/db.go`) and transaction boundary (`db/tx.go`),
  `package main`, and the `webtest` harness (the composition root for
  tests) may reference it (analyzer: `connconfine`). Mid-call-tree code
  takes `db.Queryable`.
- Use `Exec` for statements you don't read rows from (DELETE/UPDATE).
  A `Query` whose rows are never closed leaks the pooled connection.
- Run `bin/generate` after adding or removing a `FooTx`; `bin/check`
  fails on drift in the generated variants.

## Never

- **A transaction in `context.Context`.** No ambient transactions, ever:
  `pgx.Tx` is not concurrency-safe, and ctx-carried transactions hide
  that hazard while making the seam invisible. This was considered and
  deliberately rejected.
- Sharing one `pgx.Tx` across goroutines.
- Logic in the generated variant. It hands `db.Conn` to the `*Tx` form;
  that's all.

**Why:** in the project this template was extracted from, this convention
produced two shipped bugs while held by discipline alone — so here it is
held by machinery. The explicit pairing keeps transaction scope visible
at every call site, and machine-generating the pool half removes both the
duplication cost and the only place the convention could drift.
