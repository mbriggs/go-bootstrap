// Code generation entry point: models from the live dev schema, then
// pool-backed delegates from hand-written FooTx signatures (order matters —
// txgen type-checks packages, so models must be current first). Run via
// bin/generate or go generate after tern migrate; output is committed and
// bin/check fails on drift. Deliberately not wired into builds or the air
// loop: builds must never need a database.
//
//go:generate go run ./cmd/modelgen
//go:generate go run ./cmd/txgen ./...
package main
