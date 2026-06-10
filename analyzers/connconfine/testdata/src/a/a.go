package a

import "db"

func MidCallTree() *db.Pool {
	return db.Conn // want `db\.Conn referenced outside conn\.gen\.go and db bootstrap: take a db\.Queryable parameter instead`
}
