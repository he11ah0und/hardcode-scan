package scan

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
)

// scanGoFile parses path with go/parser and flags every string literal
// (token.STRING BasicLit) that contains detected runes. Comments are never
// flagged — unlike the grep-based guard this tool replaces.
// The key is the raw source line containing the literal, leading
// whitespace preserved.
func scanGoFile(root, path string, keys map[string]struct{}) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return err
	}
	lines := splitLines(src)
	var walkErr error
	ast.Inspect(f, func(n ast.Node) bool {
		if walkErr != nil {
			return false
		}
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING || !literalHasDetect(lit.Value) {
			return true
		}
		line := fset.Position(lit.Pos()).Line
		if line < 1 || line > len(lines) {
			return true
		}
		key, err := relKey(root, path, lines[line-1])
		if err != nil {
			walkErr = err
			return false
		}
		keys[key] = struct{}{}
		return true
	})
	return walkErr
}

// literalHasDetect checks both the raw literal source and its unquoted
// value (the latter catches \u0410-style escapes).
func literalHasDetect(raw string) bool {
	if HasDetectRunes(raw) {
		return true
	}
	if v, err := strconv.Unquote(raw); err == nil && HasDetectRunes(v) {
		return true
	}
	return false
}
