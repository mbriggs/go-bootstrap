// Package connconfine reports references to the db package's global
// connection pool outside its allowed homes: conngen's output (conn.gen.go),
// the db package's own bootstrap (db.go) and transaction boundary (tx.go),
// package main, and the webtest harness (the composition root for tests,
// playing main's role). Hand-written code mid-call-tree must take a
// db.Queryable instead — reaching for the pool there is how reads silently
// escape transactions. Other generated files get no exemption: templ output
// comes from hand-written sources, so a pool reference there is still a
// hand-written pool reference.
package connconfine

import (
	"go/ast"
	"go/types"
	"path/filepath"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "connconfine",
	Doc:  "reports db.Conn references outside conn.gen.go, db bootstrap, and package main",
	Run:  run,
}

// genFileName must match cmd/conngen's output filename.
const genFileName = "conn.gen.go"

var allowedDBFiles = map[string]bool{
	"db.go": true,
	"tx.go": true,
}

func run(pass *analysis.Pass) (any, error) {
	if pass.Pkg.Name() == "main" || pass.Pkg.Name() == "webtest" {
		return nil, nil
	}

	for _, file := range pass.Files {
		filename := filepath.Base(pass.Fset.Position(file.Pos()).Filename)
		if filename == genFileName {
			continue
		}

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
					"db.Conn referenced outside conn.gen.go and db bootstrap: take a db.Queryable parameter instead")
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
