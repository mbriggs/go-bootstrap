package jobconfine_test

import (
	"testing"

	"github.com/mbriggs/go-bootstrap/analyzers/jobconfine"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestJobConfine(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), jobconfine.Analyzer,
		"a", "jobs")
}
