package db

// tx.go is the transaction boundary — the other allowed home.
func InTx() *Pool {
	return Conn
}
