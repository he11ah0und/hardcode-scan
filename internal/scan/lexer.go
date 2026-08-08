package scan

// lexer.go implements a small frame-stack lexer shared by the TS/TSX
// scanner and the Svelte {...} expression scanner. It understands:
//
//   - line comments // and block comments /* ... */ (cross-line);
//   - '...' and "..." strings (single-line, escapes honored);
//   - template literals `...` (cross-line) with ${...} interpolations —
//     interpolation contents are lexed as code, not as string text, and
//     may nest (strings, templates, comments inside);
//   - detection of non-ASCII runes in code (JSX text, template text).
//
// String and template literal contents (outside interpolations) are
// collected per literal and, in latin mode, evaluated against the
// English UI-text heuristic when the literal closes.

type frameKind int

const (
	frameStr    frameKind = iota // '...' or "..."
	frameTmpl                    // `...`
	frameInterp                  // ${...} inside a template literal
	frameBlock                   // /* ... */
)

type lexFrame struct {
	kind    frameKind
	quote   rune // frameStr: closing quote
	depth   int  // frameInterp: brace nesting depth
	escaped bool
	seg     []rune // string contents (frameStr, frameTmpl)
}

// textLexer is the per-file (or per-expression) lexer state.
type textLexer struct {
	frames []lexFrame
	found  bool
	latin  bool
}

func (l *textLexer) evalSeg(f *lexFrame) {
	if l.latin && looksLikeUIText(string(f.seg)) {
		l.found = true
	}
	f.seg = f.seg[:0]
}

// endLine evaluates the pending string segment of the top frame (a
// single-line string left open at EOL, or the per-line part of a
// multi-line template literal). The frame itself stays open.
func (l *textLexer) endLine() {
	if n := len(l.frames); n > 0 {
		if f := &l.frames[n-1]; f.kind == frameStr || f.kind == frameTmpl {
			l.evalSeg(f)
		}
	}
}

// feed consumes one line rune by rune. It returns true when the rest of
// the line is a line comment (caller should stop feeding this line).
func (l *textLexer) feed(runes []rune) bool {
	for i := 0; i < len(runes); i++ {
		next := rune(0)
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		stop, skip := l.feedRune(runes[i], next)
		if stop {
			return true
		}
		if skip {
			i++
		}
	}
	return false
}

// feedRune consumes a single rune; next is the following rune (0 when the
// rune ends the line or is fed without lookahead). It returns stop=true
// when the rest of the line is a line comment, and skip=true when next
// was consumed as the second char of a two-char token (/*, */, ${).
func (l *textLexer) feedRune(r, next rune) (stop, skip bool) {
	if n := len(l.frames); n > 0 {
		f := &l.frames[n-1]
		switch f.kind {
		case frameBlock:
			if r == '*' && next == '/' {
				l.frames = l.frames[:n-1]
				return false, true
			}
		case frameStr:
			switch {
			case f.escaped:
				f.escaped = false
			case r == '\\':
				f.escaped = true
			case r == f.quote:
				l.evalSeg(f)
				l.frames = l.frames[:n-1]
			default:
				f.seg = append(f.seg, r)
				if IsDetectRune(r) {
					l.found = true
				}
			}
		case frameTmpl:
			switch {
			case f.escaped:
				f.escaped = false
			case r == '\\':
				f.escaped = true
			case r == '`':
				l.evalSeg(f)
				l.frames = l.frames[:n-1]
			case r == '$' && next == '{':
				l.frames = append(l.frames, lexFrame{kind: frameInterp, depth: 1})
				return false, true
			default:
				f.seg = append(f.seg, r)
				if IsDetectRune(r) {
					l.found = true
				}
			}
		case frameInterp:
			switch {
			case r == '/' && next == '/':
				return true, false // line comment inside interpolation
			case r == '/' && next == '*':
				l.frames = append(l.frames, lexFrame{kind: frameBlock})
				return false, true
			case r == '\'' || r == '"' || r == '`':
				kind := frameStr
				if r == '`' {
					kind = frameTmpl
				}
				l.frames = append(l.frames, lexFrame{kind: kind, quote: r})
			case r == '{':
				f.depth++
			case r == '}':
				f.depth--
				if f.depth == 0 {
					l.frames = l.frames[:n-1]
				}
			default:
				if IsDetectRune(r) {
					l.found = true
				}
			}
		}
		return false, false
	}
	// Code level.
	switch {
	case r == '/' && next == '/':
		return true, false // rest of the line is a comment
	case r == '/' && next == '*':
		l.frames = append(l.frames, lexFrame{kind: frameBlock})
		return false, true
	case r == '\'' || r == '"' || r == '`':
		kind := frameStr
		if r == '`' {
			kind = frameTmpl
		}
		l.frames = append(l.frames, lexFrame{kind: kind, quote: r})
	case IsDetectRune(r):
		l.found = true
	}
	return false, false
}
