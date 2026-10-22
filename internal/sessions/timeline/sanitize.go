package timeline

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// esc is the byte that opens every terminal escape sequence.
const esc = 0x1b

// Sanitize strips what a terminal would act on rather than print: escape
// sequences (CSI, OSC, DCS/PM/APC, or a lone ESC and the byte after it), C0
// controls other than \n and \t, DEL, C1 controls, and bidirectional
// embeddings, overrides and isolates. Event text is authored by models, tools
// and peers; rendered raw it could clear the screen, move the cursor, retitle
// the window or show text in a different order from its bytes. Everything
// else, valid UTF-8 or not, passes through untouched.
func Sanitize(s string) string {
	if clean(s) {
		return s
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == esc:
			size = escapeLen(s[i:])
		case r == '\n' || r == '\t':
			out = append(out, s[i])
		case r < 0x20 || r == 0x7f:
			// dropped
		case r >= 0x80 && r <= 0x9f || bidiControl(r):
			// dropped
		case r == utf8.RuneError && size == 1 && s[i] >= 0x80 && s[i] <= 0x9f:
			// A stray 8-bit C1 byte: invalid UTF-8, but some terminals honour it.
		default:
			out = append(out, s[i:i+size]...)
		}
		i += size
	}
	return string(out)
}

// showHidden writes whatever a terminal would act on or print as nothing as an
// escape such as \x1b or \u200b. Sanitize drops such text, up to the rest of a
// string, and the user asked to approve a call should see all of it.
func showHidden(s string) string {
	if utf8.ValidString(s) && strings.IndexFunc(s, hidden) < 0 {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1 && s[i] >= 0x80 && s[i] <= 0x9f:
			fmt.Fprintf(&b, `\x%02x`, s[i])
		case hidden(r):
			quoted := strconv.QuoteRuneToASCII(r)
			b.WriteString(quoted[1 : len(quoted)-1])
		default:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}

func hidden(r rune) bool {
	return r != '\n' && r != '\t' && unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp,
		unicode.Variation_Selector, unicode.Other_Default_Ignorable_Code_Point)
}

func bidiControl(r rune) bool {
	return r >= 0x202a && r <= 0x202e || r >= 0x2066 && r <= 0x2069
}

// clean is the no-allocation fast path. In valid UTF-8 a C1 control is
// exactly 0xC2 followed by a byte below 0xA0, and every bidirectional control
// starts with 0xE2.
func clean(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b < 0x20 && b != '\n' && b != '\t' || b == 0x7f || b == 0xc2 && i+1 < len(s) && s[i+1] < 0xa0 {
			return false
		}
		if b == 0xe2 {
			if r, _ := utf8.DecodeRuneInString(s[i:]); bidiControl(r) {
				return false
			}
		}
	}
	return true
}

// escapeLen is how many bytes the escape sequence opening s spans. String
// sequences (OSC, DCS, PM, APC) run to BEL or ST; an unterminated one
// swallows the rest of s, which is what the terminal would have done.
func escapeLen(s string) int {
	if len(s) < 2 {
		return len(s)
	}
	switch s[1] {
	case '[':
		for i := 2; i < len(s); i++ {
			if s[i] >= 0x40 && s[i] <= 0x7e {
				return i + 1
			}
		}
		return len(s)
	case ']', 'P', '^', '_':
		for i := 2; i < len(s); i++ {
			switch {
			case s[i] == 0x07:
				return i + 1
			case s[i] == esc && i+1 < len(s) && s[i+1] == '\\':
				return i + 2
			}
		}
		return len(s)
	}
	_, n := utf8.DecodeRuneInString(s[1:])
	return 1 + n
}
