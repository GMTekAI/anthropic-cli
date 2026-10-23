package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscape(t *testing.T) {
	for _, tt := range []struct{ in, escaped, lines string }{
		{in: "", escaped: "", lines: ""},
		{in: "\x1b[2J", escaped: `\x1b[2J`, lines: `\x1b[2J`},
		{in: "\x1b]52;c;aGk=\x07", escaped: `\x1b]52;c;aGk=\a`, lines: `\x1b]52;c;aGk=\a`},
		{in: "\a", escaped: `\a`, lines: `\a`},
		{in: "a\rb", escaped: `a\rb`, lines: `a\rb`},
		{in: "\x7f", escaped: `\x7f`, lines: `\x7f`},
		// The same CSI control, as a raw byte and as a UTF-8 encoded rune.
		{in: "\x9b2J", escaped: `\x9b2J`, lines: `\x9b2J`},
		{in: "\u009b2J", escaped: `\u009b2J`, lines: `\u009b2J`},
		{in: "a\xffb", escaped: `a\xffb`, lines: `a\xffb`},
		{in: "€\xe2\x82", escaped: `€\xe2\x82`, lines: `€\xe2\x82`},
		{in: "a\u202eb", escaped: `a\u202eb`, lines: `a\u202eb`},
		{in: "a\u200bb", escaped: `a\u200bb`, lines: `a\u200bb`},
		{in: "👩\u200d💻.md", escaped: "👩\u200d💻.md", lines: "👩\u200d💻.md"},
		{in: "می\u200cخواهم", escaped: "می\u200cخواهم", lines: "می\u200cخواهم"},
		{in: "a\tb", escaped: `a\tb`, lines: "a\tb"},
		{in: "a\nb", escaped: `a\nb`, lines: "a\nb"},
		{in: "日本語", escaped: "日本語", lines: "日本語"},
		{in: "€5", escaped: "€5", lines: "€5"},
		{in: "日本　語", escaped: "日本　語", lines: "日本　語"},
		// A real U+FFFD is printable, unlike the invalid byte that decodes to it.
		{in: "\ufffd", escaped: "\ufffd", lines: "\ufffd"},
		{in: `C:\new "x"`, escaped: `C:\new "x"`, lines: `C:\new "x"`},
	} {
		assert.Equal(t, tt.escaped, Escape(tt.in), "Escape(%q)", tt.in)
		assert.Equal(t, tt.lines, EscapeLines(tt.in), "EscapeLines(%q)", tt.in)
		assert.Equal(t, tt.escaped, Escape(tt.escaped), "Escape twice: %q", tt.in)
		assert.Equal(t, tt.lines, EscapeLines(tt.lines), "EscapeLines twice: %q", tt.in)
	}
}
