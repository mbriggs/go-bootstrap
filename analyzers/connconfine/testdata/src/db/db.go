// Stub of the project's db package: the analyzer matches the Conn name,
// package name, and file basenames, not the real module path. db.go is an
// allowed home for the pool.
package db

type Pool struct{}

var Conn *Pool

func Configure() {
	Conn = &Pool{}
}
