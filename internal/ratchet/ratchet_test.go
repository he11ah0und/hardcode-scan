package ratchet

import (
	"os"
	"path/filepath"
	"testing"
)

func setOf(keys ...string) map[string]struct{} {
	s := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		s[k] = struct{}{}
	}
	return s
}

func TestCompareNew(t *testing.T) {
	newKeys, stale := Compare([]string{"a.go:x", "b.go:y"}, setOf("a.go:x"))
	if len(newKeys) != 1 || newKeys[0] != "b.go:y" {
		t.Fatalf("new = %v, want [b.go:y]", newKeys)
	}
	if len(stale) != 0 {
		t.Fatalf("stale = %v, want empty", stale)
	}
}

func TestCompareStale(t *testing.T) {
	newKeys, stale := Compare([]string{"a.go:x"}, setOf("a.go:x", "gone.go:z"))
	if len(newKeys) != 0 {
		t.Fatalf("new = %v, want empty", newKeys)
	}
	if len(stale) != 1 || stale[0] != "gone.go:z" {
		t.Fatalf("stale = %v, want [gone.go:z]", stale)
	}
}

func TestCompareOK(t *testing.T) {
	newKeys, stale := Compare([]string{"a.go:x"}, setOf("a.go:x"))
	if len(newKeys) != 0 || len(stale) != 0 {
		t.Fatalf("new = %v, stale = %v, want both empty", newKeys, stale)
	}
}

func TestLoadAllowMissingFile(t *testing.T) {
	set, err := LoadAllow(filepath.Join(t.TempDir(), "missing.allow"))
	if err != nil {
		t.Fatalf("missing allowlist must not be an error: %v", err)
	}
	if len(set) != 0 {
		t.Fatalf("missing allowlist must yield an empty set, got %v", set)
	}
}

func TestLoadAllowSkipsEmptyLines(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.allow")
	if err := os.WriteFile(p, []byte("a.go:x\n\nb.go:y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	set, err := LoadAllow(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(set) != 2 {
		t.Fatalf("got %v, want 2 entries", set)
	}
}

func TestWriteAllowSortsDedupesAndRoundTrips(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.allow")
	if err := WriteAllow(p, []string{"b.go:y", "a.go:x", "b.go:y"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "a.go:x\nb.go:y\n" {
		t.Fatalf("written %q, want sorted deduped lines", data)
	}
	set, err := LoadAllow(p)
	if err != nil {
		t.Fatal(err)
	}
	newKeys, stale := Compare([]string{"a.go:x", "b.go:y"}, set)
	if len(newKeys) != 0 || len(stale) != 0 {
		t.Fatalf("round trip must compare clean, new = %v, stale = %v", newKeys, stale)
	}
}
