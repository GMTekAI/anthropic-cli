package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-cli/internal/declarative/core"
	"github.com/stretchr/testify/assert"
)

// The renderer is the last thing between a diff and a terminal or CI log. Core
// withholds secret values from the tree; the renderer must not go around it by
// printing Desired or Remote for anything but a create's summary fields.
func TestSensitiveNodesRenderAsAPlaceholder(t *testing.T) {
	var buf bytes.Buffer
	r := &Renderer{Out: &buf}
	r.renderDiff(&core.Diff{Kind: core.DiffObject, Fields: map[string]*core.Diff{
		"resources": {Kind: core.DiffList, Items: []core.ItemDiff{
			{Before: -1, After: 0, Diff: &core.Diff{Kind: core.DiffSensitive}},
			{Before: 0, After: 1, Diff: &core.Diff{Kind: core.DiffObject, Fields: map[string]*core.Diff{
				"authorization_token": {Kind: core.DiffWriteOnly},
				"url":                 {Kind: core.DiffText, Before: "https://a", After: "https://b"},
			}}},
		}},
	}}, "")

	out := buf.String()
	assert.Contains(t, out, "(sensitive; changed)")
	assert.Contains(t, out, "write-only")
	assert.Contains(t, out, `"https://a" → "https://b"`)
	assert.Contains(t, out, "[0→1]")
}

// A one-word edit in a long prompt shows the words around it, not the prompt.
func TestLongTextShowsTheEditInContext(t *testing.T) {
	before := strings.Repeat("lorem ipsum dolor sit amet ", 20) + "review the code carefully" + strings.Repeat(" consectetur adipiscing elit", 20)
	after := strings.Replace(before, "carefully", "thoroughly and kindly", 1)

	var buf bytes.Buffer
	r := &Renderer{Out: &buf}
	r.renderDiff(&core.Diff{Kind: core.DiffObject, Fields: map[string]*core.Diff{
		"system": {Kind: core.DiffText, Before: before, After: after},
	}}, "")

	out := buf.String()
	assert.Contains(t, out, "[-carefully-]{+thoroughly and kindly+}")
	assert.Contains(t, out, "review the code ")
	assert.Contains(t, out, "words…")
	assert.Less(t, len(out), len(before)/2, "most of the unchanged prose is elided")

	buf.Reset()
	r.Verbose = true
	r.renderDiff(&core.Diff{Kind: core.DiffObject, Fields: map[string]*core.Diff{
		"system": {Kind: core.DiffText, Before: before, After: after},
	}}, "")
	assert.NotContains(t, buf.String(), "words…", "verbose keeps the whole text")
}
