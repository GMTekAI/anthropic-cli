package render

import (
	"fmt"
	"io"

	"github.com/charmbracelet/x/ansi"
)

// Redrawing part of the output in place. A terminal will erase what is still
// on screen but never what has scrolled into the scrollback, so the renderer
// counts the rows it has produced since a mark — wrapped lines included — and
// only rewinds when all of them are still visible. Otherwise the caller prints
// afresh beneath, which is honest if less slick.

// rowCounter wraps the output and counts terminal rows as they are written.
type rowCounter struct {
	w        io.Writer
	viewport Viewport
	rows     int
	col      int // display cells used on the current row
}

func (c *rowCounter) Write(p []byte) (int, error) {
	width, _ := c.viewport()
	// Measure printable width line by line; escape sequences take no cells.
	start := 0
	for i, b := range p {
		if b != '\n' {
			continue
		}
		c.advance(ansi.StringWidth(string(p[start:i])), width)
		c.rows++
		c.col = 0
		start = i + 1
	}
	c.advance(ansi.StringWidth(string(p[start:])), width)
	return c.w.Write(p)
}

// advance accounts for cells written on the current row, adding a row each
// time the terminal would have wrapped.
func (c *rowCounter) advance(cells, width int) {
	c.col += cells
	if width <= 0 || cells == 0 {
		return
	}
	for c.col > width {
		c.rows++
		c.col -= width
	}
}

// Viewport reports the terminal's size. The renderer wraps long values to its
// width, and redraws in place only when everything since the rewind point fits
// its height. Without one it neither wraps nor redraws.
type Viewport func() (width, height int)

// markRewindPoint starts counting rows from here; Rewind erases back to it.
func (r *Renderer) markRewindPoint() {
	if r.counter != nil {
		r.counter.rows, r.counter.col = 0, 0
	}
}

// Rewind erases everything written since the preview began (RenderPlan marks
// that point), plus extra rows the renderer did not write itself (an echoed
// line of input), provided all of it is still on screen.
// It reports whether it did; when it could not, nothing is touched.
func (r *Renderer) Rewind(extra int) bool {
	if r.counter == nil || r.Viewport == nil || !r.Color {
		return false
	}
	_, height := r.Viewport()
	rows := r.counter.rows + extra
	if r.counter.col > 0 {
		rows++
	}
	if height <= 0 || rows >= height {
		return false
	}
	// Cursor to column 1, up N rows, clear to end of screen.
	fmt.Fprintf(r.counter.w, "\r\x1b[%dA\x1b[J", rows)
	r.markRewindPoint()
	return true
}

// out is where rendering goes: through the row counter when a viewport is
// known, straight to Out otherwise.
func (r *Renderer) out() io.Writer {
	if r.Viewport == nil {
		return r.Out
	}
	if r.counter == nil {
		r.counter = &rowCounter{w: r.Out, viewport: r.Viewport}
	}
	return r.counter
}
