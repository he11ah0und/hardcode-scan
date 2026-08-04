package scan

import (
	"os"
)

// scanTSFile flags every line whose non-comment part contains detected
// runes. String contents and JSX text count as non-comment. The key is
// the raw line, leading whitespace preserved.
//
// The scanner is a deliberately small state machine, not a full TS lexer:
//   - block comments /* ... */ are tracked across lines;
//   - line comments // end at the end of the line and are only recognised
//     outside of strings/templates;
//   - '...' and "..." strings never span lines;
//   - template literals `...` are tracked across lines and their whole
//     body — including ${...} interpolations — is treated as string
//     content, so comment markers inside an interpolation would be missed
//     (accepted simplification);
//   - regex literals are not recognised; a regex containing detected
//     runes is flagged as ordinary code.
func scanTSFile(root, path string, keys map[string]struct{}) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	st := tsState{}
	for _, line := range splitLines(src) {
		if st.lineHasDetect(line) {
			key, err := relKey(root, path, line)
			if err != nil {
				return err
			}
			keys[key] = struct{}{}
		}
	}
	return nil
}

// tsState carries the cross-line scanner state.
type tsState struct {
	inBlock    bool // inside /* ... */
	inTemplate bool // inside `...`
}

// lineHasDetect reports whether the non-comment part of line contains
// detected runes, updating the cross-line state.
func (st *tsState) lineHasDetect(line string) bool {
	runes := []rune(line)
	var quote rune // '\'' or '"' while inside a single-line string
	escaped := false
	// found records a hit; scanning continues so the cross-line state
	// (block comments, template literals) stays correct.
	found := false
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		next := rune(0)
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		switch {
		case st.inBlock:
			if r == '*' && next == '/' {
				st.inBlock = false
				i++
			}
		case st.inTemplate:
			if escaped {
				escaped = false
			} else if r == '\\' {
				escaped = true
			} else if r == '`' {
				st.inTemplate = false
			} else if IsDetectRune(r) {
				found = true
			}
		case quote != 0:
			if escaped {
				escaped = false
			} else if r == '\\' {
				escaped = true
			} else if r == quote {
				quote = 0
			} else if IsDetectRune(r) {
				found = true
			}
		default:
			switch {
			case r == '/' && next == '/':
				return found // rest of the line is a comment
			case r == '/' && next == '*':
				st.inBlock = true
				i++
			case r == '\'' || r == '"':
				quote = r
			case r == '`':
				st.inTemplate = true
			case IsDetectRune(r):
				found = true
			}
		}
	}
	return found
}
