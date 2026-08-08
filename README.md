# hardcode-scan

A ratchet guard against hardcoded user-facing text in source: UI text belongs in i18n resources, not in code. The tool scans Go, TS/TSX and Svelte, compares the result against an allowlist of known debt, and fails when the debt grows (new strings appeared) or the allowlist goes stale (strings were removed while their entries remain).

Two detection modes:

- **default** — any non-ASCII letter in any script (Cyrillic, Greek, Arabic, Hebrew, CJK, kana, Hangul, Latin with diacritics, and so on) inside string or text boundaries. Digits, punctuation and emoji are not flagged;
- **latin (opt-in per directory)** — additionally flags ASCII English text that looks like a UI string. Off by default, because code is full of legitimate English strings (logs, errors, keys), so it is enabled selectively for UI code (`internal/gui`, `frontend/src`, ...).

How it differs from a grep guard in a Makefile:

- **Go is scanned via AST** (`go/parser`): only string literals are flagged, so comments containing Cyrillic are not caught.
- **The frontend is covered**: TS/TSX go through a line-based scanner that understands `//`, `/* ... */`, `'...'`, `"..."` and multi-line template literals `` `...` ``; Svelte has a separate scanner with `<script>`/`<style>`/template zones.

## Installation

```sh
go install github.com/he11ah0und/hardcode-scan@latest
```

## Usage

```sh
hardcode-scan -root . -allow .hardcode-scan.allow \
	-go internal,cmd -ts web/src -svelte web/src \
	-latin internal/gui,web/src
```

An example Makefile target:

```make
i18n-hardcode:
	hardcode-scan -root . -allow scripts/i18n-hardcode.allow \
		-go backend/internal,backend/cmd -ts web/src -svelte web/src \
		-latin backend/internal/gui,web/src
```

Flags:

- `-root` (default `.`) — project root; paths in keys are relative to it.
- `-allow` (default `.hardcode-scan.allow`) — path to the allowlist. A missing file is treated as an empty allowlist (everything found counts as "new").
- `-go` — directories for the Go scan (relative to root, repeatable and/or comma-separated). Empty = Go is not scanned.
- `-ts` — the same for TS/TSX (`*.ts`, `*.tsx`).
- `-svelte` — the same for Svelte (`*.svelte`).
- `-latin` — directories where ASCII English UI text is additionally flagged (repeatable and/or comma-separated; `.` = everywhere). Empty = English is not searched for.
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

**Go** (`-go`): walks `*.go`, skipping `*_test.go`, hidden directories and `vendor/`. String literals containing detection runes are flagged (including through `\uXXXX` escapes). Comments are not flagged. In latin mode, literals that look like an English UI sentence are flagged (≥2 words of ≥2 letters; URLs containing `://` are excluded).

**TS/TSX** (`-ts`): walks `*.ts`/`*.tsx`, skipping hidden directories, `node_modules/` and `dist/`. A line-based state machine: a line is flagged when detection runes occur outside comments — in strings, JSX text or code. In latin mode the heuristic applies to the contents of strings and template literals; ASCII JSX text is not detected (the scanner does not distinguish JSX text from code — use the Svelte scanner for templates). Deliberate simplifications:

- the body of a template literal, interpolations `${...}` included, counts as a string throughout, so comments inside `${...}` are not recognised;
- regex literals are not recognised — a regex containing Cyrillic is flagged as ordinary code;
- `'...'` and `"..."` are never multi-line.

**Svelte** (`-svelte`): the file is split into zones. `<script>` follows the TS rules. `<style>` is skipped entirely (CSS `content:` is a deliberate blind spot). Template: text nodes between tags are UI text by definition (in latin mode text with ≥2 ASCII letters is flagged; HTML entities such as `&gt;` are skipped); strings inside `{...}` expressions follow the TS rules; attribute values are always flagged for non-ASCII, and flagged for English only in known UI attributes (`placeholder`, `title`, `alt`, `aria-label`, `label`) — otherwise `class="flex gap-2"` would flood the report. HTML comments `<!-- ... -->` are not flagged, multi-line ones included. Deliberate simplifications:

- a `</script>` inside a string within `<script>` ends the zone early;
- template literals inside `{...}` are treated as ordinary strings (no nested `${}`);
- `{...}` expressions are assumed to be single-line.

## Log and error policy

Logs and errors are technical output, not UI text: they are written in English and are **not localized** — neither through localengine nor through any other i18n engine. A localized log is the same kind of mistake as hardcoded UI text.

The tool deliberately **does not try to guess** which calls are a logger: a logger can be anything with any methods, and a "looks like a log" heuristic is not reliable. So in latin mode, log argument strings and `fmt.Errorf` are flagged like everything else — that is expected. Such strings go into the allowlist (by hand or via `-update`) as consciously permitted. The ratchet still does its job: new strings will not slip through unnoticed, and the stale check will remind you to remove entries for the ones you fixed.

Rule of thumb: enable `-latin` only for directories with UI code, not for the whole project — that keeps the allowlist short and meaningful.

## Development

Standard library only, no external dependencies. Checks:

```sh
go build ./... && go test ./... && go vet ./... && gofmt -l .
```
