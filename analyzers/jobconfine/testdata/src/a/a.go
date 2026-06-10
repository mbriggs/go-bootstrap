package a

import "github.com/riverqueue/river"

var client *river.Client[int]

func Enqueue() {
	client.Insert()       // want `plain Insert skips the caller's transaction: enqueue with Client\.InsertTx beside the state change, or jobs\.InsertStandalone when there is none`
	client.InsertMany()   // want `plain InsertMany skips the caller's transaction: enqueue with Client\.InsertTx beside the state change, or jobs\.InsertStandalone when there is none`
	client.InsertTx()
	client.InsertManyTx()
}
