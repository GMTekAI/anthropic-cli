package render

import (
	"bytes"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-cli/v2/internal/declarative/core"
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

// Keys, field names and values come from files in the repository, the
// lockfile and the API. Control characters in them must print as visible
// escapes: the only escape sequences in the output are the renderer's own
// colours and links.
func TestUntrustedTextPrintsAsEscapes(t *testing.T) {
	const title = "\x1b]0;hi\x07"
	prose := strings.Repeat("review the code with care ", 4)
	updated := &core.Change{
		Key: "./agents/a" + title + ".md", Kind: "agent", Action: core.ActionUpdate,
		Entry:   &core.LockEntry{Kind: "agent", ID: "agent_\x07x"},
		Reasons: []string{"edited\u009b2J elsewhere\r"},
		Diff: &core.Diff{Kind: core.DiffObject, Fields: map[string]*core.Diff{
			"name\x1b[2J": {Kind: core.DiffValue, Before: "old\u009b", After: "new"},
			"system":      {Kind: core.DiffText, Before: prose + "end" + title, After: prose + "end, kindly"},
			"meta":        {Kind: core.DiffAdded, After: map[string]any{"k": "v\u009b"}},
			"config": {Kind: core.DiffObject, Fields: map[string]*core.Diff{
				"inner\x1b[2J": {Kind: core.DiffValue, Before: "a", After: "b"},
			}},
		}},
	}
	wd, err := os.Getwd()
	require.NoError(t, err)
	plan := &core.Plan{
		LockfilePath: filepath.Join(wd, "repo\x1b[2J", "claude-lock.json"),
		Warnings:     []string{"warn" + title},
		Changes: []*core.Change{updated, {
			Key: "./agents/b\x1b[1A.md", Kind: "agent", Action: core.ActionCreate,
			Blocked: errors.New("blocked" + title + "\n+ ./agents/forged.md"),
		}, {
			Key: "./agents/d\tunchanged.md", Kind: "agent", Action: core.ActionCreate,
			Desired: map[string]any{"note\x1b[2J": "x"},
		}},
	}
	link := func(_ core.Kind, id string) string { return "https://console.example/agents/" + url.PathEscape(id) }

	var buf bytes.Buffer
	r := &Renderer{Out: &buf, Color: true, Link: link}
	r.FirstRun(filepath.Join(wd, "first\x1b[2J", "claude-lock.json"), OriginSummary{
		Credentials: "profile p" + title, Host: "host\x1b[2J", Organization: "org\u009b", Workspace: "ws\r",
	})
	r.Untracked("/repo\x1b[2J", []core.Found{
		{Key: "./agents/c\x1b[2J.md", Kind: "agent"},
		{Key: "./agents/e\u202egnp.md", Kind: "agent"},
	}, false)
	r.RenderPlan(plan)
	r.Expanded, r.Verbose = true, true
	r.RenderPlan(plan)
	r.Applied(core.Applied{Change: updated, ID: updated.Entry.ID, Outcome: "updated"})
	r.Applied(core.Applied{
		Change: plan.Changes[1], ID: "agent_\x07y", Outcome: "failed",
		Err: errors.New("failed"),
	})

	own := regexp.MustCompile("\x1b\\[[0-9;]*m|\x1b\\]8;;[^\x1b\x07]*\x1b\\\\")
	text := own.ReplaceAllString(buf.String(), "")
	for _, c := range text {
		if c != '\n' && !strconv.IsGraphic(c) {
			t.Fatalf("%q reached the output raw:\n%s", c, strconv.Quote(text))
		}
	}
	for _, want := range []string{
		`./first\x1b[2J/claude-lock.json does not exist yet`,
		`profile p\x1b]0;hi\a`,
		`host\x1b[2J`,
		`org\u009b`,
		`ws\r`,
		`agent_\ay`,
		`./agents/a\x1b]0;hi\a.md`,
		`./agents/b\x1b[1A.md`,
		`./agents/c\x1b[2J.md`,
		`./agents/e\u202egnp.md`,
		`./agents/d\tunchanged.md`,
		`/repo\x1b[2J — not in the lockfile`,
		`./repo\x1b[2J/claude-lock.json`,
		`note\x1b[2J: "x"`,
		`config.inner\x1b[2J: "a" → "b"`,
		`warning: warn\x1b]0;hi\a`,
		`blocked\x1b]0;hi\a` + "\n    + ./agents/forged.md",
		`edited\u009b2J elsewhere\r`,
		`name\x1b[2J`,
		`"old\u009b"`,
		`end\x1b]0;hi`,
		`{"k":"v\u009b"}`,
		`agent_\ax`,
	} {
		assert.Contains(t, text, want)
	}
}

// Ordinary text, including non-ASCII, emoji and tab-indented prose, prints as
// it did before escaping was added.
func TestPrintableTextIsUnchanged(t *testing.T) {
	prose := strings.Repeat("keep the code tidy ", 4) + "\n\tif ok {\n\t\treturn \"日本語\"\n\t}"
	plan := &core.Plan{
		LockfilePath: "/repo/claude-lock.json",
		Changes: []*core.Change{{
			Key: "./agents/日本語👩\u200d💻.md", Kind: "agent", Action: core.ActionUpdate,
			Entry: &core.LockEntry{Kind: "agent", ID: "agent_01x"},
			Diff: &core.Diff{Kind: core.DiffObject, Fields: map[string]*core.Diff{
				"description": {Kind: core.DiffValue, Before: "日本語", After: "short"},
				"system":      {Kind: core.DiffText, Before: prose, After: prose + " // done"},
			}},
		}},
	}

	var buf bytes.Buffer
	(&Renderer{Out: &buf, Expanded: true}).RenderPlan(plan)
	out := buf.String()
	assert.NotContains(t, out, `\x`)
	assert.NotContains(t, out, `\u`)
	assert.NotContains(t, out, `\"`)
	assert.Contains(t, out, "./agents/日本語👩\u200d💻.md")
	assert.Contains(t, out, "\tif ok {")
	assert.Contains(t, out, `description: "日本語" → "short"`)
}
