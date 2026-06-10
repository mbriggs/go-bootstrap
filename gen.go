// Code generation entry point: models from the live dev schema, then
// direct db.Conn variants from hand-written FooTx signatures (order matters —
// conngen type-checks packages, so models must be current first). Run via
// bin/generate or go generate after bin/migrate up; output is committed and
// bin/check fails on drift. Deliberately not wired into builds or the air
// loop: builds must never need a database.
//
//go:generate go run ./cmd/modelgen
//go:generate go run ./cmd/conngen ./...
package main
