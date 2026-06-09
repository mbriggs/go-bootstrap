package testdata

import (
	"math/rand/v2"
	"sync"
)

// DataSequence generates short random codes, unique within a run, for test
// data naming. Parallel tests rely on this uniqueness to stay row-scoped,
// so Next is safe for concurrent use.
type DataSequence struct {
	sync.Mutex
	used map[string]bool
}

func NewSequence() *DataSequence {
	return &DataSequence{
		used: make(map[string]bool),
	}
}

const maxAttempts = 1000

// Next returns a code not previously returned by this sequence.
func (ds *DataSequence) Next() string {
	ds.Lock()
	defer ds.Unlock()

	for attempts := 0; attempts < maxAttempts; attempts++ {
		val := generateRandomString(5)
		if _, exists := ds.used[val]; !exists {
			ds.used[val] = true
			return val
		}
	}

	panic("data sequence exhausted")
}

func generateRandomString(length int) string {
	const alphanumeric = "abcdefghijklmnopqrstuvwxyz0123456789"

	chars := make([]byte, length)
	for i := range chars {
		chars[i] = alphanumeric[rand.IntN(len(alphanumeric))]
	}

	return string(chars)
}
