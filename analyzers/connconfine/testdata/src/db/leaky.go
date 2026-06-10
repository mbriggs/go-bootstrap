package db

// Any other file in the db package gets no exemption.
func Leak() *Pool {
	return Conn // want `db\.Conn referenced outside conn\.gen\.go and db bootstrap: take a db\.Queryable parameter instead`
}
