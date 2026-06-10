// Package jobs is the one allowed home for plain Insert — the
// InsertStandalone wrapper lives here.
package jobs

import "github.com/riverqueue/river"

var Client *river.Client[int]

func InsertStandalone() {
	Client.Insert()
}
