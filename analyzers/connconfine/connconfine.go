// Package connconfine reports references to the db package's global
// connection pool outside its allowed homes: generated delegate files, the
// db package's own bootstrap (db.go) and transaction boundary (tx.go),
// package main, and the webtest harness (the composition root for tests,
// playing main's role). Hand-written code mid-call-tree must take a
// db.Queryable instead — reaching for the pool there is how reads silently
// escape transactions.
package connconfine

import (
	"go/ast"
	"go/types"
	"path/filepath"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "connconfine",
	Doc:  "reports db.Conn references outside generated files, db bootstrap, and package main",
	Run:  run,
}

var allowedDBFiles = map[string]bool{
	"db.go": true,
	"tx.go": true,
}

func run(pass *analysis.Pass) (any, error) {
	if pass.Pkg.Name() == "main" || pass.Pkg.Name() == "webtest" {
		return nil, nil
	}

	for _, file := range pass.Files {
		if ast.IsGenerated(file) {
			continue
		}

		filename := filepath.Base(pass.Fset.Position(file.Pos()).Filename)
		if pass.Pkg.Name() == "db" && allowedDBFiles[filename] {
			continue
		}

		ast.Inspect(file, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}

			if isPoolVar(pass.TypesInfo.Uses[id]) {
				pass.Reportf(id.Pos(),
					"db.Conn referenced outside generated delegates and db bootstrap: take a db.Queryable parameter instead")
			}

			return true
		})
	}

	return nil, nil
}

func isPoolVar(obj types.Object) bool {
	v, ok := obj.(*types.Var)
	if !ok || v.Pkg() == nil {
		return false
	}

	return v.Name() == "Conn" && v.Pkg().Name() == "db" && v.Parent() == v.Pkg().Scope()
}
