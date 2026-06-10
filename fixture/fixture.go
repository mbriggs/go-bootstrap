// Package fixture provides identity for test data: deterministic integer
// ids derived from (table, name), and run-unique random codes for naming
// rows. The id offset keeps seeded rows clear of ordinary serial-pk ranges
// so they can't collide with rows the app inserts.
package fixture

import (
	"crypto/sha256"
	"encoding/binary"
)

const offset = 1 << 28

// ID returns a stable fixture id for (table, name).
func ID(table, name string) int64 {
	sum := sha256.Sum256([]byte(table + ":" + name))
	return offset + int64(binary.BigEndian.Uint32(sum[:4]))%offset
}
