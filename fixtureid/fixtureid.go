// Package fixtureid produces deterministic integer fixture ids derived from
// (table, name). The offset keeps them clear of ordinary serial-pk ranges so
// seeded rows can't collide with rows the app inserts.
package fixtureid

import (
	"crypto/sha256"
	"encoding/binary"
)

const offset = 1 << 28

// For returns a stable fixture id for (table, name).
func For(table, name string) int64 {
	sum := sha256.Sum256([]byte(table + ":" + name))
	return offset + int64(binary.BigEndian.Uint32(sum[:4]))%offset
}
