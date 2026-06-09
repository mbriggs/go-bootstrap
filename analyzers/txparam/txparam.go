// Package txparam reports functions that accept a db.Queryable parameter
// and never use it. An ignored transaction parameter means the body reads or
// writes through some other executor — silently escaping the caller's
// transaction, which is exactly how the original FindTx bug shipped.
package txparam

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "txparam",
	Doc:  "reports db.Queryable parameters that are declared but never used",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if ast.IsGenerated(file) {
			continue
		}

		ast.Inspect(file, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				return true
			}

			checkParams(pass, fn)

			return true
		})
	}

	return nil, nil
}

func checkParams(pass *analysis.Pass, fn *ast.FuncDecl) {
	for _, field := range fn.Type.Params.List {
		t := pass.TypesInfo.TypeOf(field.Type)
		if t == nil || !isQueryable(t) {
			continue
		}

		for _, name := range field.Names {
			if name.Name == "_" {
				continue
			}

			obj := pass.TypesInfo.Defs[name]
			if obj == nil {
				continue
			}

			if !usedIn(pass, fn.Body, obj) {
				pass.Reportf(name.Pos(),
					"transaction parameter %s is never used: queries in %s will escape the caller's transaction",
					name.Name, fn.Name.Name)
			}
		}
	}
}

func usedIn(pass *analysis.Pass, body *ast.BlockStmt, obj types.Object) bool {
	used := false

	ast.Inspect(body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && pass.TypesInfo.Uses[id] == obj {
			used = true
			return false
		}

		return !used
	})

	return used
}

func isQueryable(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}

	obj := named.Obj()

	return obj.Name() == "Queryable" && obj.Pkg() != nil && obj.Pkg().Name() == "db"
}
