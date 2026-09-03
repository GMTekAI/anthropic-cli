package render

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Rewind may only erase what is still on screen. Rows are counted as the
// terminal lays them out — a long line wraps onto several — and once they no
// longer fit the viewport nothing is erased at all.
func TestRewindOnlyErasesWhatIsStillOnScreen(t *testing.T) {
	var buf bytes.Buffer
	width, height := 20, 10
	r := &Renderer{Out: &buf, Color: true, Viewport: func() (int, int) { return width, height }}

	r.markRewindPoint()
	r.line("short")                 // 1 row
	r.line(strings.Repeat("x", 45)) // wraps to 3 rows at width 20
	fmt.Fprint(r.out(), "prompt? ") // partial row
	require.True(t, r.Rewind(1), "5 rows + 1 echoed fits in 10")
	assert.True(t, strings.HasSuffix(buf.String(), "\r\x1b[6A\x1b[J"))

	buf.Reset()
	r.markRewindPoint()
	for range 12 {
		r.line("row")
	}
	assert.False(t, r.Rewind(0), "12 rows have scrolled past a 10-row viewport")
	assert.NotContains(t, buf.String(), "\x1b[J", "nothing is erased when it cannot all be")
}
