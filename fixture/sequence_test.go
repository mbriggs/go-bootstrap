package fixture_test

import (
	"sync"
	"testing"

	"github.com/mbriggs/go-bootstrap/fixture"
)

func TestSequenceNextIsUniqueUnderConcurrency(t *testing.T) {
	t.Parallel()

	seq := fixture.NewSequence()

	const workers, perWorker = 8, 50
	codes := make(chan string, workers*perWorker)

	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for range perWorker {
				codes <- seq.Next()
			}
		})
	}
	wg.Wait()
	close(codes)

	seen := make(map[string]bool)
	for code := range codes {
		if seen[code] {
			t.Fatalf("sequence repeated code %q", code)
		}
		seen[code] = true
	}
}
