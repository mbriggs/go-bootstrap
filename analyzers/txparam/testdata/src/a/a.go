package a

import "db"

func UsedTx(tx db.Queryable) {
	tx.Query()
}

func UnusedTx(tx db.Queryable) { // want `transaction parameter tx is never used: queries in UnusedTx will escape the caller's transaction`
	_ = 1
}

func BlankTx(_ db.Queryable) {}

func UsedViaCall(tx db.Queryable) {
	UsedTx(tx)
}

func NotQueryable(n int) {
	_ = n
}
