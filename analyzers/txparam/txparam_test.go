package txparam_test

import (
	"testing"

	"github.com/mbriggs/go-bootstrap/analyzers/txparam"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestTxParam(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), txparam.Analyzer, "a")
}
