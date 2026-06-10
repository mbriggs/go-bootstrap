package fixture_test

import (
	"testing"

	"github.com/mbriggs/go-bootstrap/fixture"
)

func TestIDIsDeterministic(t *testing.T) {
	t.Parallel()

	first := fixture.ID("users", "default")
	second := fixture.ID("users", "default")
	if first != second {
		t.Fatalf("same (table, name) produced different ids: %d vs %d", first, second)
	}
}

func TestIDSeparatesTablesAndNames(t *testing.T) {
	t.Parallel()

	a := fixture.ID("users", "default")
	b := fixture.ID("users", "admin")
	c := fixture.ID("things", "default")

	if a == b || a == c {
		t.Fatalf("expected distinct ids, got %d %d %d", a, b, c)
	}
}

func TestIDStaysAboveSerialRange(t *testing.T) {
	t.Parallel()

	if id := fixture.ID("users", "default"); id < 1<<28 {
		t.Fatalf("id %d below the serial-pk offset", id)
	}
}
