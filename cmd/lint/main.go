// lint runs the project's custom analyzers. These enforce the conventions
// that caused real shipped bugs when left to discipline alone.
//
// Usage: go run ./cmd/lint ./...
package main

import (
	"github.com/mbriggs/go-bootstrap/analyzers/connconfine"
	"github.com/mbriggs/go-bootstrap/analyzers/txparam"
	"golang.org/x/tools/go/analysis/multichecker"
)

func main() {
	multichecker.Main(
		txparam.Analyzer,
		connconfine.Analyzer,
	)
}
