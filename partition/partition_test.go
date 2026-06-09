package partition_test

import (
	"errors"
	"testing"

	"github.com/mbriggs/go-bootstrap/partition"
)

var errBoom = errors.New("boom")

func TestResultsReturnsEmptyFailuresSliceWhenAllRowsParse(t *testing.T) {
	values, failures := partition.Results([]int{1, 2}, func(v int) partition.ParseResult[int, error] {
		return partition.Ok[int, error](v)
	})

	if len(values) != 2 {
		t.Fatalf("values len = %d, want 2", len(values))
	}
	if failures == nil {
		t.Fatal("failures = nil, want empty slice")
	}
	if len(failures) != 0 {
		t.Fatalf("failures len = %d, want 0", len(failures))
	}
}

func TestResultsPartitionsFailures(t *testing.T) {
	values, failures := partition.Results([]int{1, 2}, func(v int) partition.ParseResult[int, error] {
		if v == 2 {
			return partition.Fail[int](errBoom)
		}
		return partition.Ok[int, error](v)
	})

	if len(values) != 1 || values[0] != 1 {
		t.Fatalf("values = %v, want [1]", values)
	}
	if len(failures) != 1 || !errors.Is(failures[0], errBoom) {
		t.Fatalf("failures = %v, want [boom]", failures)
	}
}
