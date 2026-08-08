// Package scan finds hardcoded user-facing text in Go, TS/TSX and Svelte
// sources and produces ratchet keys of the form "relative/path:raw line
// content" (no line numbers, leading whitespace preserved — same semantics
// as the grep-based guard it replaces).
//
// Two detection modes exist:
//   - default: any non-ASCII letter (any script — Cyrillic, Greek, Arabic,
//     CJK, kana, hangul, ...) inside string/text boundaries is flagged;
//   - latin (opt-in per directory): ASCII English text that looks like a UI
//     string is flagged as well. This is off by default because code is
//     full of legitimate English strings (logs, errors, keys).
package scan

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// HasDetectRunes reports whether s contains any non-ASCII letter, i.e. a
// rune from any script other than plain ASCII Latin (Cyrillic, Greek,
// Arabic, Hebrew, CJK, kana, hangul, accented Latin, ...).
func HasDetectRunes(s string) bool {
	for _, r := range s {
		if IsDetectRune(r) {
			return true
		}
	}
	return false
}

// IsDetectRune reports whether r is a letter outside the ASCII range.
// Digits, punctuation, symbols and emoji are never detected; the
// Letterlike Symbols block (U+2100–U+214F) is excluded as well — its
// members (ℹ, ℡, ...) are letter-category by Unicode but are used as
// icons, not as text.
func IsDetectRune(r rune) bool {
	if r >= 0x2100 && r <= 0x214F {
		return false
	}
	return r > 0x7F && unicode.IsLetter(r)
}

// looksLikeUIText reports whether s looks like a user-facing English
// sentence or label: at least two words of two or more ASCII letters.
// URLs and single identifiers/keys are excluded so that logs, i18n keys
// and technical strings do not flag.
func looksLikeUIText(s string) bool {
	if strings.Contains(s, "://") {
		return false
	}
	words := 0
	for _, w := range strings.Fields(s) {
		letters := 0
		for _, r := range w {
			if r <= 0x7F && unicode.IsLetter(r) {
				letters++
			}
		}
		if letters >= 2 {
			words++
		}
	}
	return words >= 2
}

// hasASCIILetters reports whether s contains at least n ASCII letters.
func hasASCIILetters(s string, n int) bool {
	count := 0
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			count++
			if count >= n {
				return true
			}
		}
	}
	return false
}

// Scan walks goDirs for Go sources, tsDirs for TS/TSX sources and
// svelteDirs for Svelte sources (all relative to root) and returns the
// sorted, deduplicated ratchet keys. Empty dir lists disable the
// corresponding scan. latinDirs (also relative to root) enable latin
// detection for files inside them; "." enables it everywhere.
func Scan(root string, goDirs, tsDirs, svelteDirs, latinDirs []string) ([]string, error) {
	keys := map[string]struct{}{}
	latin := func(path string) bool { return inDirs(root, path, latinDirs) }
	for _, dir := range goDirs {
		if err := walkSources(root, dir, goSource, keys, latin, scanGoFile); err != nil {
			return nil, err
		}
	}
	for _, dir := range tsDirs {
		if err := walkSources(root, dir, tsSource, keys, latin, scanTSFile); err != nil {
			return nil, err
		}
	}
	for _, dir := range svelteDirs {
		if err := walkSources(root, dir, svelteSource, keys, latin, scanSvelteFile); err != nil {
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

// inDirs reports whether path (a file under root) lies inside any of the
// given root-relative directories. An empty list matches nothing; "."
// matches everything under root.
func inDirs(root, path string, dirs []string) bool {
	if len(dirs) == 0 {
		return false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	for _, dir := range dirs {
		dir = strings.TrimSuffix(filepath.ToSlash(dir), "/")
		if dir == "." {
			return true
		}
		if rel == dir || strings.HasPrefix(rel, dir+"/") {
			return true
		}
	}
	return false
}

// fileScanner scans one file. latin enables ASCII English UI-text
// detection for this file.
type fileScanner func(root, path string, keys map[string]struct{}, latin bool) error

var skipDirs = map[string]struct{}{
	"node_modules": {},
	"vendor":       {},
	"dist":         {},
}

func walkSources(root, dir string, match func(name string) bool, keys map[string]struct{}, latin func(path string) bool, scan fileScanner) error {
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
		return scan(root, path, keys, latin(path))
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

func svelteSource(name string) bool {
	return strings.HasSuffix(name, ".svelte")
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
