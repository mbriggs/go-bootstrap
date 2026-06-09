package db

import (
	"reflect"

	"github.com/mbriggs/pgsql"
)

// FilterUnset removes zero-valued entries for the given columns so their
// database defaults apply instead of being overwritten with Go zero values.
func FilterUnset(rm pgsql.RowMap, columns ...string) {
	for _, col := range columns {
		v, ok := rm[col]
		if !ok {
			continue
		}

		if v == nil || reflect.ValueOf(v).IsZero() {
			delete(rm, col)
		}
	}
}
