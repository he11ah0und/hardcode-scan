// hardcode-scan is a ratchet guard against hardcoded non-ASCII (and,
// opt-in, English) UI text in Go, TS/TSX and Svelte sources. See
// README.md for usage.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/he11ah0und/hardcode-scan/internal/ratchet"
	"github.com/he11ah0und/hardcode-scan/internal/scan"
)

// dirList is a repeatable, comma-separated string flag.
type dirList []string

func (d *dirList) String() string { return strings.Join(*d, ",") }

func (d *dirList) Set(v string) error {
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			*d = append(*d, part)
		}
	}
	return nil
}

func main() {
	var (
		root       = flag.String("root", ".", "project root; key paths are relative to it")
		allow      = flag.String("allow", ".hardcode-scan.allow", "allowlist file path")
		goDirs     dirList
		tsDirs     dirList
		svelteDirs dirList
		latinDirs  dirList
		update     = flag.Bool("update", false, "overwrite the allowlist with the current scan")
		quiet      = flag.Bool("quiet", false, "suppress the OK message on success")
	)
	flag.Var(&goDirs, "go", "directories to scan for Go sources (repeatable, comma-separated; relative to root)")
	flag.Var(&tsDirs, "ts", "directories to scan for TS/TSX sources (repeatable, comma-separated; relative to root)")
	flag.Var(&svelteDirs, "svelte", "directories to scan for Svelte sources (repeatable, comma-separated; relative to root)")
	flag.Var(&latinDirs, "latin", "directories where ASCII English UI text is also flagged (repeatable, comma-separated; \".\" = everywhere)")
	flag.Parse()

	keys, err := scan.Scan(*root, goDirs, tsDirs, svelteDirs, latinDirs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hardcode-scan: scan failed: %v\n", err)
		os.Exit(2)
	}

	if *update {
		if err := ratchet.WriteAllow(*allow, keys); err != nil {
			fmt.Fprintf(os.Stderr, "hardcode-scan: cannot write allowlist: %v\n", err)
			os.Exit(2)
		}
		fmt.Printf("hardcode-scan: wrote %d keys to %s\n", len(keys), *allow)
		return
	}

	allowSet, err := ratchet.LoadAllow(*allow)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hardcode-scan: cannot read allowlist: %v\n", err)
		os.Exit(2)
	}

	newKeys, stale := ratchet.Compare(keys, allowSet)
	failed := false
	if len(newKeys) > 0 {
		fmt.Println("New hardcoded non-ASCII strings — move user-facing texts to i18n resources:")
		for _, k := range newKeys {
			fmt.Println(k)
		}
		fmt.Printf("(Intentional data, not UI text? Add the line to %s or run with -update)\n", *allow)
		failed = true
	}
	if len(stale) > 0 {
		fmt.Printf("Stale allowlist entries (string fixed or moved) — remove them from %s:\n", *allow)
		for _, k := range stale {
			fmt.Println(k)
		}
		failed = true
	}
	if failed {
		os.Exit(1)
	}
	if !*quiet {
		fmt.Println("hardcode-scan: OK (no new hardcoded strings)")
	}
}
