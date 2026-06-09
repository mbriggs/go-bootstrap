package web

import (
	"sync"
	"time"
)

// Signin throttling: bcrypt's cost slows offline cracking; this slows
// online guessing. Keys are (client IP, email) so one address can't walk
// the user list and one user can't be locked out from everywhere.
const (
	throttleLimit  = 5
	throttleWindow = 15 * time.Minute
)

// ThrottleStore tracks failed signin attempts per key. The default is
// in-memory and per-process; a horizontally scaled deployment replaces
// SigninThrottle at boot with an implementation backed by shared state
// (e.g. Postgres or Redis) — otherwise throttling silently stops working
// across instances.
type ThrottleStore interface {
	// Blocked reports whether key has reached the failure limit.
	Blocked(key string) bool
	// Fail records one failed attempt for key.
	Fail(key string)
	// Reset clears key after a successful signin.
	Reset(key string)
}

// SigninThrottle is consulted by SigninSubmit. Swap it before the router
// is built.
var SigninThrottle ThrottleStore = &memoryThrottle{failures: map[string][]time.Time{}}

type memoryThrottle struct {
	mu       sync.Mutex
	failures map[string][]time.Time
}

func (t *memoryThrottle) Blocked(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return len(t.prune(key)) >= throttleLimit
}

func (t *memoryThrottle) Fail(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.failures[key] = append(t.prune(key), time.Now())
}

func (t *memoryThrottle) Reset(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.failures, key)
}

// prune drops entries older than the window. Caller holds mu.
func (t *memoryThrottle) prune(key string) []time.Time {
	cutoff := time.Now().Add(-throttleWindow)

	var kept []time.Time
	for _, at := range t.failures[key] {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}

	if len(kept) == 0 {
		delete(t.failures, key)
	} else {
		t.failures[key] = kept
	}

	return kept
}
