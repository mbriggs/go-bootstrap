package a

import "db"

// conngen output is the designed home for pool references.
func Generated() *db.Pool {
	return db.Conn
}
