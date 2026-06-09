package fixtureid_test

import (
	"testing"

	"github.com/mbriggs/go-bootstrap/fixtureid"
)

func TestForIsDeterministic(t *testing.T) {
	t.Parallel()

	first := fixtureid.For("users", "default")
	second := fixtureid.For("users", "default")
	if first != second {
		t.Fatalf("same (table, name) produced different ids: %d vs %d", first, second)
	}
}

func TestForSeparatesTablesAndNames(t *testing.T) {
	t.Parallel()

	a := fixtureid.For("users", "default")
	b := fixtureid.For("users", "admin")
	c := fixtureid.For("things", "default")

	if a == b || a == c {
		t.Fatalf("expected distinct ids, got %d %d %d", a, b, c)
	}
}

func TestForStaysAboveSerialRange(t *testing.T) {
	t.Parallel()

	if id := fixtureid.For("users", "default"); id < 1<<28 {
		t.Fatalf("id %d below the serial-pk offset", id)
	}
}
