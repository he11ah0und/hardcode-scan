package scan

import (
	"os"
	"strings"
)

// scanSvelteFile scans a .svelte component. The file is split into three
// zones, each with its own rules:
//   - <script>: scanned with the TS state machine (strings, templates,
//     comments); latin mode applies the English UI-text heuristic to
//     string contents;
//   - <style>: skipped entirely (CSS content: is accepted as a blind
//     spot);
//   - template: text nodes between tags are user-facing text by
//     definition and are flagged when they contain detected runes or, in
//     latin mode, two or more ASCII letters; quoted strings inside {...}
//     expressions (both between and inside tags) follow the TS string
//     rules; attribute values are flagged for detected runes in any
//     attribute and, in latin mode, for ASCII text in known UI attributes
//     only (class etc. would flood the report otherwise). HTML comments
//     <!-- ... --> are tracked across lines and never flagged.
//
// Deliberate simplifications: <script>/<style> are recognised anywhere on
// a line, but </script> inside a script string would end the zone early;
// template literals inside {...} expressions are treated like plain
// strings (no ${} nesting); {...} expressions never span lines.
func scanSvelteFile(root, path string, keys map[string]struct{}, latin bool) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	st := svelteState{zone: zoneTemplate}
	for _, line := range splitLines(src) {
		if st.lineHasDetect(line, latin) {
			key, err := relKey(root, path, line)
			if err != nil {
				return err
			}
			keys[key] = struct{}{}
		}
	}
	return nil
}

const (
	zoneTemplate = iota
	zoneScript
	zoneStyle
)

// uiAttrs are the attributes whose ASCII values count as user-facing text
// in latin mode.
var uiAttrs = map[string]struct{}{
	"placeholder": {},
	"title":       {},
	"alt":         {},
	"aria-label":  {},
	"label":       {},
}

// svelteState carries the cross-line scanner state.
type svelteState struct {
	zone        int
	htmlComment bool // inside <!-- ... -->
	inTag       bool // inside <...> in the template zone
	attrQuote   rune // quote opening the current attribute value
	attrName    []rune
	attrVal     []rune
	nameBuf     []rune
	ts          tsState
}

// lineHasDetect processes one source line, possibly switching zones
// mid-line, and reports whether it contains flagged text.
func (st *svelteState) lineHasDetect(line string, latin bool) bool {
	found := false
	rest := line
	for len(rest) > 0 {
		switch st.zone {
		case zoneScript:
			if idx := strings.Index(rest, "</script>"); idx >= 0 {
				if st.ts.lineHasDetect(rest[:idx], latin) {
					found = true
				}
				st.ts = tsState{}
				st.zone = zoneTemplate
				rest = rest[idx+len("</script>"):]
				continue
			}
			if st.ts.lineHasDetect(rest, latin) {
				found = true
			}
			return found
		case zoneStyle:
			if idx := strings.Index(rest, "</style>"); idx >= 0 {
				st.zone = zoneTemplate
				rest = rest[idx+len("</style>"):]
				continue
			}
			return found
		default:
			var f bool
			rest, f = st.scanTemplateFragment(rest, latin)
			if f {
				found = true
			}
		}
	}
	return found
}

// exprScanner consumes a {...} expression rune by rune. Quoted strings
// inside are collected per literal and checked for detected runes and (in
// latin mode) the English UI-text heuristic; detected runes anywhere else
// in the expression flag as well.
type exprScanner struct {
	depth   int
	quote   rune
	escaped bool
	seg     []rune
	found   bool
}

// step feeds one rune; done reports that the closing '}' of the outermost
// brace was consumed.
func (e *exprScanner) step(r rune, latin bool) (done bool) {
	switch {
	case e.quote != 0:
		switch {
		case e.escaped:
			e.escaped = false
		case r == '\\':
			e.escaped = true
		case r == e.quote:
			e.quote = 0
			if latin && looksLikeUIText(string(e.seg)) {
				e.found = true
			}
			e.seg = e.seg[:0]
		default:
			e.seg = append(e.seg, r)
			if IsDetectRune(r) {
				e.found = true
			}
		}
	case r == '\'' || r == '"' || r == '`':
		e.quote = r
	case r == '{':
		e.depth++
	case r == '}':
		e.depth--
		if e.depth == 0 {
			return true
		}
	default:
		if IsDetectRune(r) {
			e.found = true
		}
	}
	return false
}

// scanTemplateFragment scans template-zone text until the end of the line
// or until a <script>/<style> tag switches the zone. It returns the
// unprocessed remainder of the line (empty when the whole line was
// consumed) and whether flagged text was found.
func (st *svelteState) scanTemplateFragment(line string, latin bool) (string, bool) {
	runes := []rune(line)
	found := false
	// textBuf accumulates text-node characters for the latin heuristic.
	var textBuf []rune
	flushText := func() {
		if latin && hasASCIILetters(string(textBuf), 2) {
			found = true
		}
		textBuf = textBuf[:0]
	}
	// expr is the active {...} expression (text level or inside a tag);
	// expressions are line-local by design.
	var expr *exprScanner

	flushAttr := func() {
		val := string(st.attrVal)
		if HasDetectRunes(val) {
			found = true
		} else if latin && hasASCIILetters(val, 2) {
			if _, ok := uiAttrs[strings.ToLower(string(st.attrName))]; ok {
				found = true
			}
		}
		st.attrName, st.attrVal = nil, nil
	}

	i := 0
	for i < len(runes) {
		r := runes[i]
		next := rune(0)
		if i+1 < len(runes) {
			next = runes[i+1]
		}

		// HTML comments run across zones and lines.
		if st.htmlComment {
			if r == '-' && next == '-' && i+2 < len(runes) && runes[i+2] == '>' {
				st.htmlComment = false
				i += 2
			}
			i++
			continue
		}

		// An active {...} expression swallows everything up to its
		// closing brace — including '=>' arrows and quotes that would
		// otherwise look like tag syntax.
		if expr != nil {
			if expr.step(r, latin) {
				if expr.found {
					found = true
				}
				expr = nil
			}
			i++
			continue
		}

		// Inside a tag: attributes, {...} expressions and the closing '>'.
		if st.inTag {
			switch {
			case st.attrQuote != 0:
				if r == st.attrQuote {
					st.attrQuote = 0
					flushAttr()
				} else {
					st.attrVal = append(st.attrVal, r)
					if IsDetectRune(r) {
						found = true
					}
				}
			case r == '{':
				expr = &exprScanner{depth: 1}
			case r == '>':
				st.inTag = false
				st.nameBuf = st.nameBuf[:0]
			case r == '=':
				st.attrName = append(st.attrName[:0], st.nameBuf...)
				st.nameBuf = st.nameBuf[:0]
			case r == '\'' || r == '"':
				st.attrQuote = r
				st.attrVal = st.attrVal[:0]
			case isAttrNameRune(r):
				st.nameBuf = append(st.nameBuf, r)
			default:
				st.nameBuf = st.nameBuf[:0]
			}
			i++
			continue
		}

		// Text level.
		switch {
		case r == '<' && next == '!' && i+3 < len(runes) && runes[i+2] == '-' && runes[i+3] == '-':
			flushText()
			st.htmlComment = true
			i += 4
		case r == '<' && matchesTagOpen(runes, i, "script"):
			flushText()
			st.zone = zoneScript
			return string(runes[skipTag(runes, i):]), found
		case r == '<' && matchesTagOpen(runes, i, "style"):
			flushText()
			st.zone = zoneStyle
			return string(runes[skipTag(runes, i):]), found
		case r == '<':
			flushText()
			st.inTag = true
			i++
		case r == '&':
			// Skip HTML entities (&gt; &#160; ...) so their letters do
			// not count as text.
			if n := entityLen(runes[i:]); n > 0 {
				i += n
			} else {
				textBuf = append(textBuf, r)
				i++
			}
		case r == '{':
			flushText()
			expr = &exprScanner{depth: 1}
			i++
		default:
			textBuf = append(textBuf, r)
			if IsDetectRune(r) {
				found = true
			}
			i++
		}
	}
	if expr != nil && expr.found {
		found = true
	}
	flushText()
	return "", found
}

// entityLen returns the length of the HTML entity starting at runes[0]
// ('&' included, ';' included), or 0 if runes does not start with a
// well-formed entity (&name; &#123; &#x1F; with at least one body char).
func entityLen(runes []rune) int {
	if len(runes) < 3 || runes[0] != '&' {
		return 0
	}
	for i := 1; i < len(runes) && i <= 10; i++ {
		r := runes[i]
		if r == ';' {
			if i > 1 {
				return i + 1
			}
			return 0
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '#' {
			continue
		}
		return 0
	}
	return 0
}

// isAttrNameRune reports whether r can be part of an attribute name
// (including svelte's on:click / bind:value forms).
func isAttrNameRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '-' || r == '_' || r == ':'
}

// matchesTagOpen reports whether runes[i:] starts with "<tag" followed by
// a name boundary (whitespace, '>' or '/').
func matchesTagOpen(runes []rune, i int, tag string) bool {
	if i+len(tag)+1 > len(runes) {
		return false
	}
	if string(runes[i+1:i+1+len(tag)]) != tag {
		return false
	}
	j := i + 1 + len(tag)
	if j >= len(runes) {
		return true // tag continues on the next line
	}
	r := runes[j]
	return r == '>' || r == '/' || r == ' ' || r == '\t'
}

// skipTag returns the index just past the '>' closing the tag that starts
// at runes[i] ('<'). Quotes are honored so '>' inside an attribute value
// does not end the tag. If the tag spans lines, the rest of this line is
// consumed and the tag state is (incorrectly but harmlessly) abandoned —
// script/style tags with attributes practically never span lines.
func skipTag(runes []rune, i int) int {
	var quote rune
	for i++; i < len(runes); i++ {
		r := runes[i]
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case '>':
			return i + 1
		}
	}
	return len(runes)
}
