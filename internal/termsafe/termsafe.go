// Package termsafe makes text from outside the program, such as API responses,
// safe to print on a terminal.
package termsafe

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	// joiners are ordinary text in emoji sequences and in scripts such as Persian.
	joiners = "‌‍"
	layout  = joiners + "\n\t"
)

// Stdout and Stderr are where plain text gets printed. Response data goes through the output formats instead,
// which render it from JSON. Like fmt.Println, they write to whatever os.Stdout and os.Stderr are at the time.
var (
	Stdout = Writer{stdStream{&os.Stdout}}
	Stderr = Writer{stdStream{&os.Stderr}}
)

type stdStream struct {
	file **os.File
}

func (s stdStream) Write(p []byte) (int, error) {
	return (*s.file).Write(p)
}

// Writer applies Lines to everything written to it. Styled output can't go through it, because the styling would be
// escaped too.
type Writer struct {
	W io.Writer
}

func (w Writer) Write(p []byte) (int, error) {
	if _, err := io.WriteString(w.W, Lines(string(p))); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Escape returns s with each non-graphic rune written as visible text, such as
// \x1b for ESC or \n for a newline, and each invalid byte as \xNN. Quotes,
// backslashes and joiners are unchanged, so ordinary text comes back as it was
// and escaping twice gives the same result as escaping once.
func Escape(s string) string {
	return escape(s, joiners)
}

// Lines is Escape for multi-line text: it also keeps newlines and tabs.
func Lines(s string) string {
	return escape(s, layout)
}

func escape(s, keep string) string {
	needsEscape := func(r rune) bool { return !strconv.IsGraphic(r) && !strings.ContainsRune(keep, r) }
	if utf8.ValidString(s) && strings.IndexFunc(s, needsEscape) < 0 {
		return s
	}
	var out strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&out, `\x%02x`, s[i])
		case needsEscape(r):
			quoted := strconv.QuoteRuneToGraphic(r)
			out.WriteString(quoted[1 : len(quoted)-1])
		default:
			out.WriteString(s[i : i+size])
		}
		i += size
	}
	return out.String()
}

// JSON rewrites encoded JSON so that no key or string in it decodes to text that Lines would change: such a rune
// becomes the escaped text instead, so `\u001b` becomes `\\x1b`. Valid JSON stays valid. JSON works on the text
// without parsing it, so a malformed document that a lenient decoder still reads comes out just as safe.
func JSON(doc string) string {
	var out strings.Builder
	out.Grow(len(doc))
	for len(doc) > 0 {
		decoded, size := decodeJSONChar(doc)
		visible := Lines(decoded)
		switch encoded := doc[:size]; {
		// Valid JSON holds a raw CR only as whitespace, where an escape would make it invalid. Dropping it instead
		// could join the two halves of a surrogate pair.
		case encoded == "\r":
			out.WriteByte(' ')
		case encoded == `\`:
			out.WriteString(`\\`)
		case visible == decoded:
			out.WriteString(encoded)
		default:
			out.WriteString(strings.ReplaceAll(visible, `\`, `\\`))
		}
		doc = doc[size:]
	}
	return out.String()
}

var jsonEscapes = map[byte]string{'"': `"`, '\\': `\`, '/': "/", 'b': "\b", 'f': "\f", 'n': "\n", 'r': "\r", 't': "\t"}

// decodeJSONChar returns what the first character of doc decodes to and how many bytes encode it. A backslash that
// starts no valid escape decodes to itself, alone.
func decodeJSONChar(doc string) (string, int) {
	if doc[0] != '\\' {
		_, size := utf8.DecodeRuneInString(doc)
		return doc[:size], size
	}
	if len(doc) > 1 {
		if decoded, ok := jsonEscapes[doc[1]]; ok {
			return decoded, 2
		}
	}
	r, ok := decodeJSONUnicode(doc)
	if !ok {
		return `\`, 1
	}
	if low, ok := decodeJSONUnicode(doc[6:]); ok && utf16.IsSurrogate(r) {
		if pair := utf16.DecodeRune(r, low); pair != utf8.RuneError {
			return string(pair), 12
		}
	}
	return string(r), 6
}

func decodeJSONUnicode(doc string) (rune, bool) {
	if len(doc) < 6 || doc[:2] != `\u` {
		return 0, false
	}
	r, err := strconv.ParseUint(doc[2:6], 16, 32)
	return rune(r), err == nil
}
