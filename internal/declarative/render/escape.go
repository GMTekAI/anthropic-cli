package render

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	// joiners are ordinary text in emoji sequences and in scripts such as Persian.
	joiners = "\u200c\u200d"
	layout  = joiners + "\n\t"
)

// Escape returns s with each control or invisible character written out as
// visible text: ESC becomes \x1b, BEL becomes \a, a newline becomes \n, and
// other such characters become escapes such as \r, \x7f or \u009b. A byte
// that is not valid UTF-8 becomes \xNN. Printable characters and joiners are
// kept, so ordinary text comes back unchanged, and escaping twice gives the
// same result as escaping once.
func Escape(s string) string { return escape(s, joiners) }

// EscapeLines is Escape for multi-line text: it keeps line breaks and tabs.
func EscapeLines(s string) string { return escape(s, layout) }

func escape(s, keep string) string {
	needsEscape := func(r rune) bool { return !strconv.IsGraphic(r) && !strings.ContainsRune(keep, r) }
	if utf8.ValidString(s) && strings.IndexFunc(s, needsEscape) < 0 {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && n == 1:
			fmt.Fprintf(&b, "\\x%02x", s[i])
		case needsEscape(r):
			q := strconv.QuoteRuneToGraphic(r)
			b.WriteString(q[1 : len(q)-1])
		default:
			b.WriteString(s[i : i+n])
		}
		i += n
	}
	return b.String()
}
