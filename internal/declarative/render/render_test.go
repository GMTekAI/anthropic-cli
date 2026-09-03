package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-cli/internal/declarative/core"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSummaryListsOnlyNonZeroCounts(t *testing.T) {
	var buf bytes.Buffer
	r := &Renderer{Out: &buf}
	r.summary("to create", "to update", "to remove", 2, 0, 0, 3)
	out := buf.String()
	assert.Contains(t, out, "+ 2 to create · 3 unchanged")
	assert.NotContains(t, out, "update")
	assert.NotContains(t, out, "remove")
	assert.NotContains(t, strings.TrimSpace(out), "\n", "the summary is a single line")
}

func TestIDsBecomeHyperlinksOnlyOnAColourTerminal(t *testing.T) {
	link := func(_ core.Kind, id string) string { return "https://console.example/agents/" + id }
	applied := core.Applied{Change: &core.Change{Key: "./agents/a.md", Kind: "agent", Action: core.ActionCreate}, ID: "agent_01x", Outcome: "created"}

	var plain bytes.Buffer
	(&Renderer{Out: &plain, Link: link}).Applied(applied)
	assert.NotContains(t, plain.String(), "\x1b]8;", "a piped log must stay free of escapes")
	assert.Contains(t, plain.String(), "agent_01x")

	var tty bytes.Buffer
	(&Renderer{Out: &tty, Color: true, Link: link}).Applied(applied)
	assert.Contains(t, tty.String(), "\x1b]8;;https://console.example/agents/agent_01x\x1b\\")
	stripped := ansi.Strip(tty.String())
	require.Contains(t, stripped, "agent_")
	assert.Equal(t, "agent_01x", strings.TrimSpace(stripped[strings.Index(stripped, "agent_"):]),
		"stripped of escapes the row reads exactly as before")
}
