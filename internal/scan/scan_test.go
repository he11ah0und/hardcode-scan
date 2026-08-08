package scan

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile creates a source file under root and returns its path.
func writeFile(t *testing.T, root, name, content string) string {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func scanDir(t *testing.T, root string, goDirs, tsDirs, svelteDirs, latinDirs []string) []string {
	t.Helper()
	keys, err := Scan(root, goDirs, tsDirs, svelteDirs, latinDirs)
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

func TestGoScanDetectsCyrillicLiteral(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "internal/seed/seed.go", "package seed\n\nvar items = map[string]string{\n\t\"a\": \"Привет\",\n\t\"b\": \"你好\",\n}\n")
	keys := scanDir(t, root, []string{"internal"}, nil, nil, nil)
	want := []string{
		"internal/seed/seed.go:\t\"a\": \"Привет\",",
		"internal/seed/seed.go:\t\"b\": \"你好\",",
	}
	if len(keys) != len(want) {
		t.Fatalf("got %v, want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Errorf("key %d = %q, want %q", i, keys[i], want[i])
		}
	}
}

func TestGoScanIgnoresComments(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "pkg/a.go", "package pkg\n\n// Комментарий с кириллицей\n/* Ещё один 注释 */\nvar ok = \"fine\"\n")
	if keys := scanDir(t, root, []string{"pkg"}, nil, nil, nil); len(keys) != 0 {
		t.Fatalf("comments must not be flagged, got %v", keys)
	}
}

func TestGoScanSkipsTestFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "pkg/a_test.go", "package pkg\n\nvar ru = \"Тест\"\n")
	if keys := scanDir(t, root, []string{"pkg"}, nil, nil, nil); len(keys) != 0 {
		t.Fatalf("_test.go must be skipped, got %v", keys)
	}
}

func TestGoScanDetectsEscapedUnicode(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "pkg/a.go", "package pkg\n\nvar ru = \"\\u041f\\u0440\\u0438\\u0432\\u0435\\u0442\"\n")
	keys := scanDir(t, root, []string{"pkg"}, nil, nil, nil)
	if len(keys) != 1 {
		t.Fatalf("escaped Cyrillic literal must be flagged, got %v", keys)
	}
}

func TestTSScanDetectsCyrillicString(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/app.ts", "const label = \"Привет\";\nconst ok = \"hello\";\n")
	keys := scanDir(t, root, nil, []string{"src"}, nil, nil)
	if len(keys) != 1 || keys[0] != "src/app.ts:const label = \"Привет\";" {
		t.Fatalf("got %v", keys)
	}
}

func TestTSScanDetectsJSXText(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/app.tsx", "export const App = () => <button>Сохранить</button>;\n")
	keys := scanDir(t, root, nil, []string{"src"}, nil, nil)
	if len(keys) != 1 {
		t.Fatalf("JSX text must be flagged, got %v", keys)
	}
}

func TestTSScanIgnoresLineComment(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/a.ts", "// комментарий с кириллицей\nconst x = 1; // ещё один 注释\n")
	if keys := scanDir(t, root, nil, []string{"src"}, nil, nil); len(keys) != 0 {
		t.Fatalf("line comments must not be flagged, got %v", keys)
	}
}

func TestTSScanIgnoresBlockComment(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/a.ts", "/*\n * Многострочный комментарий\n * с кириллицей и 汉字\n */\nconst ok = \"fine\";\n")
	if keys := scanDir(t, root, nil, []string{"src"}, nil, nil); len(keys) != 0 {
		t.Fatalf("block comments must not be flagged, got %v", keys)
	}
}

func TestTSScanDetectsAfterBlockCommentOnSameLine(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/a.ts", "/* ok */ const ru = \"Привет\";\n")
	if keys := scanDir(t, root, nil, []string{"src"}, nil, nil); len(keys) != 1 {
		t.Fatalf("string after a closed block comment must be flagged, got %v", keys)
	}
}

func TestTSScanMultilineTemplateLiteral(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/a.ts", "const msg = `первая строка\nвторая строка`;\n// после шаблона кириллица\n")
	keys := scanDir(t, root, nil, []string{"src"}, nil, nil)
	want := []string{
		"src/a.ts:const msg = `первая строка",
		"src/a.ts:вторая строка`;",
	}
	if len(keys) != len(want) {
		t.Fatalf("got %v, want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Errorf("key %d = %q, want %q", i, keys[i], want[i])
		}
	}
}

func TestTSScanCommentMarkersInsideStrings(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/a.ts", "const url = \"https://пример.рф\";\nconst s = 'a /* не комментарий */ Привет';\n")
	keys := scanDir(t, root, nil, []string{"src"}, nil, nil)
	if len(keys) != 2 {
		t.Fatalf("// and /* */ inside strings are not comments, got %v", keys)
	}
}

func TestTSScanEscapedQuote(t *testing.T) {
	root := t.TempDir()
	// The " after \" closes nothing; кириллица is outside the string but
	// the line is still real code — it must be flagged.
	writeFile(t, root, "src/a.ts", "const s = \"a \\\" quote\"; const t = \"Привет\";\n")
	if keys := scanDir(t, root, nil, []string{"src"}, nil, nil); len(keys) != 1 {
		t.Fatalf("escaped quotes must not confuse the scanner, got %v", keys)
	}
}

func TestScanSkipsNodeModulesAndDotDirs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/node_modules/dep/index.ts", "const ru = \"Привет\";\n")
	writeFile(t, root, "src/.hidden/a.ts", "const ru = \"Привет\";\n")
	if keys := scanDir(t, root, nil, []string{"src"}, nil, nil); len(keys) != 0 {
		t.Fatalf("node_modules and dot dirs must be skipped, got %v", keys)
	}
}

func TestScanEmptyDirLists(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.go", "package main\nvar ru = \"Привет\"\n")
	if keys := scanDir(t, root, nil, nil, nil, nil); len(keys) != 0 {
		t.Fatalf("empty dir lists must scan nothing, got %v", keys)
	}
}

// TestDetectRuneScripts checks that every script is detected while ASCII,
// digits, punctuation and emoji are not.
func TestDetectRuneScripts(t *testing.T) {
	detect := []struct {
		name string
		r    rune
	}{
		{"cyrillic", 'Ж'},
		{"cyrillic extended", 'Ԁ'}, // U+0500, outside the old U+0400–U+04FF range
		{"cyrillic palochka", 'ӏ'},
		{"greek", 'Ω'},
		{"arabic", 'ع'},
		{"hebrew", 'א'},
		{"hangul", '한'},
		{"hiragana", 'あ'},
		{"katakana", 'ア'},
		{"han", '汉'},
		{"devanagari", 'अ'},
		{"thai", 'ก'},
		{"georgian", 'გ'},
		{"armenian", 'Հ'},
		{"latin accented", 'é'},
	}
	for _, tc := range detect {
		if !IsDetectRune(tc.r) {
			t.Errorf("%s (%U) must be detected", tc.name, tc.r)
		}
	}
	ignore := []struct {
		name string
		r    rune
	}{
		{"ascii letter", 'Z'},
		{"digit", '7'},
		{"cyrillic-looking ascii", 'P'},
		{"punctuation", '«'},
		{"arrow", '→'},
		{"emoji", '😀'},
		{"cjk punctuation", '。'},
		{"letterlike symbol", 'ℹ'},
	}
	for _, tc := range ignore {
		if IsDetectRune(tc.r) {
			t.Errorf("%s (%U) must NOT be detected", tc.name, tc.r)
		}
	}
}
