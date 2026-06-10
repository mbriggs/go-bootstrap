package connconfine_test

import (
	"testing"

	"github.com/mbriggs/go-bootstrap/analyzers/connconfine"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestConnConfine(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), connconfine.Analyzer,
		"a", "db", "mainpkg", "webtest")
}
