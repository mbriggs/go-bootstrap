// Package webtest plays main's role for tests — exempt.
package webtest

import "db"

func Main() {
	_ = db.Conn
}
