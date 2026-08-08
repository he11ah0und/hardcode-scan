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
//
// In latin mode, ASCII string literals that look like user-facing English
// text (see looksLikeUIText) are flagged as well.
func scanGoFile(root, path string, keys map[string]struct{}, latin bool) error {
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
		if !ok || lit.Kind != token.STRING || !literalHasDetect(lit.Value, latin) {
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
// value (the latter catches \u0410-style escapes). In latin mode the
// unquoted value is also tested against the English UI-text heuristic.
func literalHasDetect(raw string, latin bool) bool {
	if HasDetectRunes(raw) {
		return true
	}
	if v, err := strconv.Unquote(raw); err == nil {
		if HasDetectRunes(v) {
			return true
		}
		if latin && looksLikeUIText(v) {
			return true
		}
	}
	return false
}
