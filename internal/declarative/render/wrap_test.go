package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A wrapped value keeps its shape: continuation lines fit the width, and a
// style open at the break is closed before it and reopened after, so nothing
// bleeds into the hanging indent.
func TestWrapStyledClosesAndReopensStylesAtBreaks(t *testing.T) {
	// Raw SGR rather than lipgloss, which renders nothing without a terminal.
	styled := "plain words then \x1b[42m\x1b[97ma long inserted phrase that must wrap somewhere in its middle\x1b[0m and done"
	lines := wrapStyled(styled, 30, 26)
	require.Greater(t, len(lines), 2)
	for i, l := range lines {
		limit := 26
		if i == 0 {
			limit = 30
		}
		assert.LessOrEqual(t, ansi.StringWidth(l), limit, "line %d: %q", i, l)
	}
	// The break inside the green phrase: line ends reset, next line reopens.
	var inside int
	for i, l := range lines {
		if strings.Contains(ansi.Strip(l), "inserted") {
			inside = i
		}
	}
	assert.True(t, strings.HasSuffix(lines[inside], "\x1b[0m"), "%q", lines[inside])
	assert.True(t, strings.HasPrefix(lines[inside+1], "\x1b[42m\x1b[97m"), "%q", lines[inside+1])
	// And the words survive intact.
	var plain []string
	for _, l := range lines {
		plain = append(plain, ansi.Strip(l))
	}
	assert.Equal(t, ansi.Strip(styled), strings.Join(plain, " "))
}
