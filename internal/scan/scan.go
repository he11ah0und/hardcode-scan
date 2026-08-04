// Package scan finds hardcoded non-ASCII (Cyrillic/CJK) text in Go and
// TS/TSX sources and produces ratchet keys of the form
// "relative/path:raw line content" (no line numbers, leading whitespace
// preserved — same semantics as the grep-based guard it replaces).
package scan

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// HasDetectRunes reports whether s contains Cyrillic (U+0400–U+04FF) or
// Han (U+4E00–U+9FFF) runes.
func HasDetectRunes(s string) bool {
	for _, r := range s {
		if IsDetectRune(r) {
			return true
		}
	}
	return false
}

// IsDetectRune reports whether r is in the detected ranges.
func IsDetectRune(r rune) bool {
	return (r >= 0x0400 && r <= 0x04FF) || (r >= 0x4E00 && r <= 0x9FFF)
}

// Scan walks goDirs for Go sources and tsDirs for TS/TSX sources (all
// relative to root) and returns the sorted, deduplicated ratchet keys.
// Empty dir lists disable the corresponding scan.
func Scan(root string, goDirs, tsDirs []string) ([]string, error) {
	keys := map[string]struct{}{}
	for _, dir := range goDirs {
		if err := walkSources(root, dir, goSource, keys, scanGoFile); err != nil {
			return nil, err
		}
	}
	for _, dir := range tsDirs {
		if err := walkSources(root, dir, tsSource, keys, scanTSFile); err != nil {
			return nil, err
		}
	}
	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

type fileScanner func(root, path string, keys map[string]struct{}) error

var skipDirs = map[string]struct{}{
	"node_modules": {},
	"vendor":       {},
	"dist":         {},
}

func walkSources(root, dir string, match func(name string) bool, keys map[string]struct{}, scan fileScanner) error {
	base := filepath.Join(root, dir)
	return filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != base && (strings.HasPrefix(name, ".") || inSet(skipDirs, name)) {
				return filepath.SkipDir
			}
			return nil
		}
		if !match(d.Name()) {
			return nil
		}
		return scan(root, path, keys)
	})
}

func inSet(set map[string]struct{}, s string) bool {
	_, ok := set[s]
	return ok
}

func goSource(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

func tsSource(name string) bool {
	return strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".tsx")
}

// relKey builds a ratchet key: root-relative slash path + ":" + raw line.
func relKey(root, path, rawLine string) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel) + ":" + rawLine, nil
}

// splitLines splits src into raw lines without trailing \n or \r.
func splitLines(src []byte) []string {
	lines := strings.Split(string(src), "\n")
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	return lines
}
