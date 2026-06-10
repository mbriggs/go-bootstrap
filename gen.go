// Code generation entry point: models from the live dev schema, then
// direct db.Conn variants from hand-written FooTx signatures. Both
// generators are syntax/schema-driven and never type-check the tree, so
// generation works even while callers of about-to-be-generated code are
// broken. Run via bin/generate or go generate after bin/migrate up; output
// is committed and bin/check fails on drift. Deliberately not wired into
// builds or the air loop: builds must never need a database.
//
//go:generate go run ./cmd/modelgen
//go:generate go run ./cmd/conngen ./...
package main
