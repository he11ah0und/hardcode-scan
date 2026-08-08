package scan

import "testing"

func TestSvelteScriptZone(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/App.svelte", "<script>\n  const label = \"Привет\";\n  // комментарий с кириллицей\n  const ok = \"fine\";\n</script>\n<p>plain</p>\n")
	keys := scanDir(t, root, nil, nil, []string{"src"}, nil)
	if len(keys) != 1 || keys[0] != "src/App.svelte:  const label = \"Привет\";" {
		t.Fatalf("got %v", keys)
	}
}

func TestSvelteTemplateText(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/App.svelte", "<p>Всего конфигов</p>\n<button>Save</button>\n")
	keys := scanDir(t, root, nil, nil, []string{"src"}, nil)
	if len(keys) != 1 || keys[0] != "src/App.svelte:<p>Всего конфигов</p>" {
		t.Fatalf("got %v", keys)
	}
}

func TestSvelteTemplateTextLatin(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/App.svelte", "<p>Total configs</p>\n<span>&gt;</span>\n")
	// Without -latin: English template text is not flagged.
	if keys := scanDir(t, root, nil, nil, []string{"src"}, nil); len(keys) != 0 {
		t.Fatalf("got %v", keys)
	}
	keys := scanDir(t, root, nil, nil, []string{"src"}, []string{"src"})
	if len(keys) != 1 || keys[0] != "src/App.svelte:<p>Total configs</p>" {
		t.Fatalf("latin template text must be flagged, got %v", keys)
	}
}

func TestSvelteIgnoresHTMLComments(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/App.svelte", "<!-- комментарий с кириллицей\nпродолжение 汉字 -->\n<p>fine</p>\n")
	if keys := scanDir(t, root, nil, nil, []string{"src"}, nil); len(keys) != 0 {
		t.Fatalf("HTML comments must not be flagged, got %v", keys)
	}
}

func TestSvelteSkipsStyleZone(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/App.svelte", "<style>\n  .a::after { content: \"привет\"; }\n</style>\n<p>fine</p>\n")
	if keys := scanDir(t, root, nil, nil, []string{"src"}, nil); len(keys) != 0 {
		t.Fatalf("style zone must be skipped, got %v", keys)
	}
}

func TestSvelteExpressionStrings(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/App.svelte", "<p>{flag ? 'Да' : 'Нет'}</p>\n")
	keys := scanDir(t, root, nil, nil, []string{"src"}, nil)
	if len(keys) != 1 {
		t.Fatalf("strings inside {...} must be flagged, got %v", keys)
	}
}

func TestSvelteAttributes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/App.svelte", "<input placeholder=\"Введите имя\" class=\"flex\">\n<input placeholder=\"Search items\" class=\"привет\">\n")
	// Non-ASCII in any attribute value flags (both placeholder and class).
	keys := scanDir(t, root, nil, nil, []string{"src"}, nil)
	if len(keys) != 2 {
		t.Fatalf("non-ASCII attribute values must be flagged, got %v", keys)
	}
}

func TestSvelteAttributesLatin(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/App.svelte", "<input placeholder=\"Search items\" class=\"flex gap two\">\n")
	// latin: only known UI attributes flag; class does not.
	keys := scanDir(t, root, nil, nil, []string{"src"}, []string{"src"})
	if len(keys) != 1 {
		t.Fatalf("latin must flag only UI attributes, got %v", keys)
	}
}

func TestSvelteInlineScriptTag(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/App.svelte", "<p>hi</p><script>const a = \"Привет\";</script><p>Конец</p>\n")
	keys := scanDir(t, root, nil, nil, []string{"src"}, nil)
	if len(keys) != 1 {
		t.Fatalf("zone switches mid-line must work, got %v", keys)
	}
}

func TestSvelteArrowFnInAttribute(t *testing.T) {
	root := t.TempDir()
	// '=>' inside a tag expression must not close the tag early; nothing
	// here is user-facing text.
	writeFile(t, root, "src/App.svelte", "<Button onclick={() => navigate(item.id)}>x</Button>\n")
	if keys := scanDir(t, root, nil, nil, []string{"src"}, []string{"src"}); len(keys) != 0 {
		t.Fatalf("arrow functions in attributes must not flag, got %v", keys)
	}
}

func TestSvelteUnionTypeInScript(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/App.svelte", "<script lang=\"ts\">\n  let { mode = 'add' }: { mode?: 'add' | 'edit' } = $props();\n</script>\n")
	if keys := scanDir(t, root, nil, nil, []string{"src"}, []string{"src"}); len(keys) != 0 {
		t.Fatalf("one-word union members must not combine into a sentence, got %v", keys)
	}
}

func TestSvelteMultilineExpression(t *testing.T) {
	root := t.TempDir()
	// Multi-line {...} expression: continuation lines must not be treated
	// as text nodes (latin FP), and strings inside must still flag.
	writeFile(t, root, "src/App.svelte", "{cond\n  ? tValue($locale, 'a.b')\n  : tValue($locale, 'c.d')}\n")
	if keys := scanDir(t, root, nil, nil, []string{"src"}, []string{"src"}); len(keys) != 0 {
		t.Fatalf("multi-line expression with keys must not flag, got %v", keys)
	}
	writeFile(t, root, "src/B.svelte", "{cond\n  ? 'Да'\n  : 'Нет'}\n")
	keys := scanDir(t, root, nil, nil, []string{"src"}, nil)
	if len(keys) != 2 {
		t.Fatalf("multi-line expression strings must flag both lines, got %v", keys)
	}
}

func TestSvelteScriptTemplateInterpolation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/App.svelte", "<script>\n  const s = `${formatSpeed(x)} ms`;\n</script>\n")
	if keys := scanDir(t, root, nil, nil, []string{"src"}, []string{"src"}); len(keys) != 0 {
		t.Fatalf("interpolation code in script must not flag, got %v", keys)
	}
}

func TestSvelteMultilineTemplateAttr(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/App.svelte", "<input\n  placeholder=\"Введите\n  имя\">\n")
	keys := scanDir(t, root, nil, nil, []string{"src"}, nil)
	if len(keys) != 2 {
		t.Fatalf("multi-line attribute value must flag both lines, got %v", keys)
	}
}

func TestSvelteTemplateLiteralInterpolationInExpression(t *testing.T) {
	root := t.TempDir()
	// Regression: runes were fed to the expression lexer one at a time
	// without lookahead, so ${ was never recognised and interpolation
	// code accumulated as template text, tripping the latin heuristic.
	writeFile(t, root, "src/App.svelte", "{@render Row(tValue($locale, 'a.b'), `${formatSpeed(c.down)} (${formatBytes(c.total)})`)}\n")
	if keys := scanDir(t, root, nil, nil, []string{"src"}, []string{"src"}); len(keys) != 0 {
		t.Fatalf("interpolation code in template expression must not flag, got %v", keys)
	}
	// Text between interpolations is still real UI text and must flag.
	writeFile(t, root, "src/B.svelte", "{x(`${a} per page ${b}`)}\n")
	if keys := scanDir(t, root, nil, nil, []string{"src"}, []string{"src"}); len(keys) != 1 {
		t.Fatalf("text between interpolations must flag, got %v", keys)
	}
}
