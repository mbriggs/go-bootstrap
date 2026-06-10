// Package main is the composition root — exempt.
package main

import "db"

func main() {
	_ = db.Conn
}
