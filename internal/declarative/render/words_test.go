package render

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiffWordsRoundTrips(t *testing.T) {
	before := "The quick brown fox\njumps over the lazy dog."
	after := "The quick red fox\nleaps over the dog!"
	runs, ok := diffWords(before, after)
	require.True(t, ok)

	var gotBefore, gotAfter strings.Builder
	for _, r := range runs {
		if r.op != wordIns {
			gotBefore.WriteString(r.text)
		}
		if r.op != wordDel {
			gotAfter.WriteString(r.text)
		}
	}
	assert.Equal(t, before, gotBefore.String())
	assert.Equal(t, after, gotAfter.String())
}
