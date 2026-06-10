package fixture

import (
	"math/rand/v2"
	"sync"
)

// Sequence generates short random codes, unique within a run, for test
// data naming. Parallel tests rely on this uniqueness to stay row-scoped,
// so Next is safe for concurrent use.
type Sequence struct {
	sync.Mutex
	used map[string]bool
}

func NewSequence() *Sequence {
	return &Sequence{
		used: make(map[string]bool),
	}
}

const maxAttempts = 1000

// Next returns a code not previously returned by this sequence.
func (s *Sequence) Next() string {
	s.Lock()
	defer s.Unlock()

	for range maxAttempts {
		val := generateRandomString(5)
		if _, exists := s.used[val]; !exists {
			s.used[val] = true
			return val
		}
	}

	panic("data sequence exhausted")
}

func generateRandomString(length int) string {
	const alphanumeric = "abcdefghijklmnopqrstuvwxyz0123456789"

	chars := make([]byte, length)
	for i := range chars {
		chars[i] = alphanumeric[rand.IntN(len(alphanumeric))] //nolint:gosec // codes name test rows; uniqueness, not unpredictability
	}

	return string(chars)
}
