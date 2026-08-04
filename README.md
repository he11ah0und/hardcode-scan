# hardcode-scan

A ratchet guard against hardcoded Cyrillic (U+0400–U+04FF) and CJK (Han, U+4E00–U+9FFF) in source: user-facing text belongs in i18n resources, not in code. The tool scans Go and TS/TSX, compares the result against an allowlist of known debt, and fails when the debt grows (new strings appeared) or the allowlist goes stale (strings were removed while their entries remain).

How it differs from a grep guard in a Makefile:

- **Go is scanned via AST** (`go/parser`): only string literals are flagged, so comments containing Cyrillic are not caught.
- **The frontend is covered**: TS/TSX go through a line-based scanner that understands `//`, `/* ... */`, `'...'`, `"..."` and multi-line template literals `` `...` ``.

## Installation

```sh
go install github.com/he11ah0und/hardcode-scan@latest
```

## Usage

```sh
hardcode-scan -root . -allow .hardcode-scan.allow -go internal,cmd -ts web/src
```

An example Makefile target:

```make
i18n-hardcode:
	hardcode-scan -root . -allow scripts/i18n-hardcode.allow \
		-go backend/internal,backend/cmd -ts web/src
```

Flags:

- `-root` (default `.`) — project root; paths in keys are relative to it.
- `-allow` (default `.hardcode-scan.allow`) — path to the allowlist. A missing file is treated as an empty allowlist (everything found counts as "new").
- `-go` — directories for the Go scan (relative to root, repeatable and/or comma-separated). Empty = Go is not scanned.
- `-ts` — the same for TS/TSX (`*.ts`, `*.tsx`).
- `-update` — rewrite the allowlist with the current scan and exit with code 0.
- `-quiet` — do not print the OK message on success.

Exit codes: `0` — clean (or `-update`), `1` — new/stale entries present, `2` — scan or I/O error.

## Ratchet semantics

The key of each finding is `relative/path:raw string contents`. The line number is **not** part of the key, so edits elsewhere in the file do not invalidate the allowlist. Leading tabs and spaces are preserved (as with grep). Duplicate keys collapse (the semantics of `sort -u`).

Comparing a scan against the allowlist:

- **new** — the key is in the scan but not in the allowlist → listed on stdout, exit 1.
- **stale** — the key is in the allowlist but is no longer found → listed on stdout, exit 1.
- otherwise — `hardcode-scan: OK (no new hardcoded strings)`, exit 0.

## Growing the allowlist

If a string is deliberate data rather than UI text (seed data with translations, for example), add the printed line to the allowlist by hand or regenerate the whole file:

```sh
hardcode-scan -root . -allow scripts/i18n-hardcode.allow \
	-go backend/internal,backend/cmd -ts web/src -update
```

`-update` rewrites the file with the sorted current scan, so stale entries drop out automatically. Debt is better paid down than allowlisted: move the texts into i18n resources — the stale check will remind you to remove the entry.

## Scanner details

**Go** (`-go`): walks `*.go`, skipping `*_test.go`, hidden directories and `vendor/`. String literals containing detection runes are flagged (including through `\uXXXX` escapes). The key is the source line of the file in which the literal starts. Comments are not flagged.

**TS/TSX** (`-ts`): walks `*.ts`/`*.tsx`, skipping hidden directories, `node_modules/` and `dist/`. A line-based state machine: a line is flagged when detection runes occur outside comments — in strings, JSX text or code. Deliberate simplifications:

- the body of a template literal, interpolations `${...}` included, counts as a string throughout, so comments inside `${...}` are not recognised;
- regex literals are not recognised — a regex containing Cyrillic is flagged as ordinary code;
- `'...'` and `"..."` are never multi-line.

## Development

Standard library only, no external dependencies. Checks:

```sh
go build ./... && go test ./... && go vet ./... && gofmt -l .
```
