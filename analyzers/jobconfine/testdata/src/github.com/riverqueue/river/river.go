// Stub of River's client: the analyzer matches the import path, the Client
// receiver, and Insert-prefixed method names — signatures don't matter.
package river

type Client[TTx any] struct{}

func (c *Client[TTx]) Insert()       {}
func (c *Client[TTx]) InsertMany()   {}
func (c *Client[TTx]) InsertTx()     {}
func (c *Client[TTx]) InsertManyTx() {}
