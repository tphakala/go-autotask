//go:build ruleguard

// Package gorules holds a rules file that ruleguard cannot load: the match
// pattern is not a valid Go expression. rules/check-fixtures.sh adds it to a
// copy of .golangci.yaml and expects lint to fail, which pins failOn: all.
package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

func Broken(m dsl.Matcher) {
	m.Match(`$x +`).Report("never loaded")
}
