package scan

import (
	"os"
)

// scanTSFile flags every line whose non-comment part contains detected
// runes. String contents and JSX text count as non-comment. The key is
// the raw line, leading whitespace preserved.
//
// In latin mode each string literal and template literal is additionally
// tested against the English UI-text heuristic (looksLikeUIText) when it
// closes; literals are evaluated independently so unrelated one-word
// strings (union types, keys) do not combine into a fake sentence, and
// ${...} interpolations are lexed as code, not as text. JSX text in
// ASCII English is not detected: the scanner cannot reliably tell JSX
// text from code — use the Svelte scanner for template text.
//
// The scanner is a deliberately small frame-stack lexer (see lexer.go),
// not a full TS parser:
//   - regex literals are not recognised; a regex containing detected
//     runes is flagged as ordinary code;
//   - '...' and "..." strings never span lines.
func scanTSFile(root, path string, keys map[string]struct{}, latin bool) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	st := tsState{}
	st.lex.latin = latin
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

// tsState carries the cross-line lexer state.
type tsState struct {
	lex textLexer
}

// lineHasDetect reports whether the non-comment part of line contains
// detected runes (or, in latin mode, English UI text inside strings),
// updating the cross-line state.
func (st *tsState) lineHasDetect(line string) bool {
	st.lex.found = false
	st.lex.feed([]rune(line))
	st.lex.endLine()
	return st.lex.found
}
