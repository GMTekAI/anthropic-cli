package render

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Soft-wrapping a styled value with a hanging indent. Left to the terminal, a
// long value wraps to column one and the shape of `key: value` is lost; done
// here, every continuation line starts one level in from the key. The text
// carries SGR escapes (colour, strikethrough), so a break has to close the
// active style before the newline and reopen it after the indent, or a green
// background bleeds across the margin.

// wrapStyled breaks s into lines of at most firstWidth (then restWidth)
// printable cells, at spaces where it can. Each returned line is
// self-contained: styles active at a break are reset at the end of one line
// and re-established at the start of the next.
func wrapStyled(s string, firstWidth, restWidth int) []string {
	if firstWidth <= 0 || restWidth <= 0 || ansi.StringWidth(s) <= firstWidth {
		return []string{s}
	}
	const reset = "\x1b[0m"
	var (
		lines  []string
		line   strings.Builder // current line, escapes included
		cells  int             // printable width of line
		active string          // SGR sequences in force since the last reset
		limit  = firstWidth
		// The last breakable point on the current line: byte offset of the
		// space in line, and the style active there.
		spaceAt     = -1
		spaceActive string
	)
	flush := func(upto int, uptoActive, carry string) {
		out := line.String()[:upto]
		if uptoActive != "" {
			out += reset
		}
		lines = append(lines, out)
		line.Reset()
		line.WriteString(uptoActive)
		line.WriteString(carry)
		cells = ansi.StringWidth(carry)
		limit = restWidth
		spaceAt = -1
	}

	state := byte(ansi.NormalState)
	for len(s) > 0 {
		seq, w, n, next := ansi.DecodeSequence(s, state, nil)
		state, s = next, s[n:]
		if w == 0 {
			// A control sequence: copy through, track SGR state, no cells.
			line.WriteString(seq)
			if strings.HasPrefix(seq, "\x1b[") && strings.HasSuffix(seq, "m") {
				if seq == reset || seq == "\x1b[m" {
					active = ""
				} else {
					active += seq
				}
			}
			continue
		}
		if seq == " " {
			if cells+w > limit {
				// Break exactly here; the space itself is dropped.
				flush(line.Len(), active, "")
				continue
			}
			spaceAt, spaceActive = line.Len(), active
		} else if cells+w > limit && cells > 0 {
			if spaceAt >= 0 {
				// Break at the last space; carry what followed it.
				carry := line.String()[spaceAt+1:]
				flush(spaceAt, spaceActive, carry)
			} else {
				flush(line.Len(), active, "")
			}
		}
		line.WriteString(seq)
		cells += w
	}
	if line.Len() > 0 {
		lines = append(lines, line.String())
	}
	return lines
}

// wrapLine prints prefix+body, wrapping body so continuation lines start at
// hang. Without a known terminal width it prints one line and lets the
// terminal do what it will.
func (r *Renderer) wrapLine(prefix, hang, body string) {
	width := 0
	if r.Viewport != nil {
		width, _ = r.Viewport()
	}
	prefixWidth, hangWidth := ansi.StringWidth(prefix), ansi.StringWidth(hang)
	// Under about 24 columns a hanging indent leaves too little room to
	// read, so the terminal's own wrapping is the better choice.
	if width <= 0 || width-hangWidth < 24 {
		r.line(prefix + body)
		return
	}
	// Leave the last column empty. A line that fills the width exactly makes
	// some terminals wrap, and the row counter would miss that row.
	lines := wrapStyled(body, width-prefixWidth-1, width-hangWidth-1)
	r.line(prefix + lines[0])
	for _, l := range lines[1:] {
		r.line(hang + l)
	}
}
