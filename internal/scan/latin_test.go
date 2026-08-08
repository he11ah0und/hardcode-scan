package scan

import "testing"

func TestLooksLikeUIText(t *testing.T) {
	ui := []string{
		"Core update",
		"Download failed",
		"Install update?",
		"avg 12 ms over 3 nodes",
		"Update complete. Please restart.",
	}
	for _, s := range ui {
		if !looksLikeUIText(s) {
			t.Errorf("%q must look like UI text", s)
		}
	}
	notUI := []string{
		"config",
		"configs.title",
		"https://example.com/path with spaces",
		"http",
		"x",
		"%w",
		"a",
	}
	for _, s := range notUI {
		if looksLikeUIText(s) {
			t.Errorf("%q must NOT look like UI text", s)
		}
	}
}

func TestGoLatinMode(t *testing.T) {
	root := t.TempDir()
	src := "package gui\n\nfunc f() {\n\tprintln(\"Core update\")\n\tprintln(\"config updated\")\n\tprintln(\"ok\")\n}\n"
	writeFile(t, root, "internal/gui/a.go", src)
	writeFile(t, root, "internal/core/b.go", src)

	// Without -latin: English strings are not flagged anywhere.
	if keys := scanDir(t, root, []string{"internal"}, nil, nil, nil); len(keys) != 0 {
		t.Fatalf("without latin dirs English must not be flagged, got %v", keys)
	}

	// With -latin on internal/gui: only that dir is flagged.
	keys := scanDir(t, root, []string{"internal"}, nil, nil, []string{"internal/gui"})
	want := []string{
		"internal/gui/a.go:\tprintln(\"Core update\")",
		"internal/gui/a.go:\tprintln(\"config updated\")",
	}
	if len(keys) != len(want) {
		t.Fatalf("got %v, want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Errorf("key %d = %q, want %q", i, keys[i], want[i])
		}
	}

	// "." enables latin everywhere.
	keys = scanDir(t, root, []string{"internal"}, nil, nil, []string{"."})
	if len(keys) != 4 {
		t.Fatalf("latin \".\" must flag both files, got %v", keys)
	}
}

func TestTSLatinMode(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/a.ts", "toast(\"Update available\");\nconst key = \"configs.title\";\nconst url = \"https://example.com/a b\";\n")
	if keys := scanDir(t, root, nil, []string{"src"}, nil, nil); len(keys) != 0 {
		t.Fatalf("without latin dirs English must not be flagged, got %v", keys)
	}
	keys := scanDir(t, root, nil, []string{"src"}, nil, []string{"src"})
	if len(keys) != 1 || keys[0] != "src/a.ts:toast(\"Update available\");" {
		t.Fatalf("got %v", keys)
	}
}

func TestTSLatinUnionType(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/a.ts", "type Mode = 'add' | 'edit';\nlet x: \"default\" | \"sm\";\n")
	if keys := scanDir(t, root, nil, []string{"src"}, nil, []string{"src"}); len(keys) != 0 {
		t.Fatalf("one-word union members must not combine into a sentence, got %v", keys)
	}
}

func TestTSLatinTemplateLiteral(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/a.ts", "const msg = `Update available for ${name}`;\n")
	keys := scanDir(t, root, nil, []string{"src"}, nil, []string{"src"})
	if len(keys) != 1 {
		t.Fatalf("latin template literal must be flagged, got %v", keys)
	}
}
