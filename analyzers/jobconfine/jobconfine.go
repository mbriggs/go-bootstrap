// Package jobconfine reports calls to the River client's non-transactional
// insert methods outside the jobs package. A plain Insert beside a state
// change is a silent correctness bug: the rows commit but the rollback
// path can still discard them independently — enqueue must ride the same
// transaction via InsertTx. Call sites with genuinely no accompanying
// state change say so by calling jobs.InsertStandalone; the name is the
// claim, and the jobs package is the one place plain Insert may live.
package jobconfine

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "jobconfine",
	Doc:  "reports river Client.Insert/InsertMany (non-Tx) outside the jobs package",
	Run:  run,
}

const riverPkgPath = "github.com/riverqueue/river"

func run(pass *analysis.Pass) (any, error) {
	if pass.Pkg.Name() == "jobs" {
		return nil, nil
	}

	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			if isPlainRiverInsert(pass.TypesInfo.Uses[sel.Sel]) {
				pass.Reportf(call.Pos(),
					"plain %s skips the caller's transaction: enqueue with Client.InsertTx beside the state change, or jobs.InsertStandalone when there is none",
					sel.Sel.Name)
			}

			return true
		})
	}

	return nil, nil
}

// isPlainRiverInsert reports whether obj is a non-Tx insert method on
// river's Client. Matching Insert* without the Tx suffix keeps future
// river insert variants covered.
func isPlainRiverInsert(obj types.Object) bool {
	fn, ok := obj.(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != riverPkgPath {
		return false
	}

	if !strings.HasPrefix(fn.Name(), "Insert") || strings.HasSuffix(fn.Name(), "Tx") {
		return false
	}

	recv := fn.Signature().Recv()
	if recv == nil {
		return false
	}

	named := namedOf(recv.Type())

	return named != nil && named.Obj().Name() == "Client"
}

func namedOf(t types.Type) *types.Named {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	named, _ := t.(*types.Named)

	return named
}
