package timeline

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		"clear\x1b[2Jscreen":                       "clearscreen",
		"\x1b[31mred\x1b[0m":                       "red",
		"\x1b]0;title\x07after":                    "after",
		"\x1b]8;;http://x\x1b\\link\x1b]8;;\x1b\\": "link",
		"\x1bPdcs\x1b\\x":                          "x",
		"lone\x1b":                                 "lone",
		"esc\x1bZpair":                             "escpair",
		"over\rwrite":                              "overwrite",
		"back\b\bspace":                            "backspace",
		"bell\x07":                                 "bell",
		"c1\xc2\x9b2Jcsi":                          "c12Jcsi",
		"del\x7f":                                  "del",
		"stray\x9bbyte":                            "straybyte",
		"keep\ttabs\nand lines":                    "keep\ttabs\nand lines",
		"emoji 🙂 and 日本語":                          "emoji 🙂 and 日本語",
		"unterminated\x1b]0;swallows the rest":     "unterminated",
		"over\u202eride\u202c, em\u202abed":        "override, embed",
		"iso\u2066la\u2067t\u2068e\u2069":          "isolate",
		"עברית with a \u200f mark":                 "עברית with a \u200f mark",
	}
	for in, want := range cases {
		require.Equal(t, want, Sanitize(in), "%q", in)
	}
}

// Nothing in a tool call is dropped or invisible, above all while the user is
// asked to approve it.
func TestToolCallsShowHiddenCharacters(t *testing.T) {
	unseen := "ls\U000e003b\U000e0072\U000e006d /a\u200bb\u2060\ufeff \u202ec\u2069\u2028d \u00ade\ufe0f\U000e0101\u3164\x7f\u009b"
	unseenShown := `ls\U000e003b\U000e0072\U000e006d /a\u200bb\u2060\ufeff \u202ec\u2069\u2028d \u00ade\ufe0f\U000e0101\u3164\x7f\u009b`
	actedOn := " \x1b[2Jf\r\x1b]0; rm -rf /"
	shown := unseenShown + ` \x1b[2Jf\r\x1b]0; rm -rf /`
	bash := use(t, "agent.tool_use", "bash", `{"command":`+js(unseen+actedOn)+`}`, "")
	mcp := use(t, "agent.mcp_tool_use", "q\u200b", "{\"k\":\""+unseen+"\"}", `"mcp_server_name":"db\u202e"`)
	colored := ev(t, `{"id":"r1","type":"agent.tool_result","tool_use_id":"u1","content":[{"type":"text","text":`+js("\x1b[31mred\x1b[0m 👍\ufe0f\r")+`}]}`)

	brief := use(t, "agent.tool_use", "bash", `{"command":`+js("ls\u200b\x1b]0; rm -rf /")+`,"description":`+js("list\U000e003b\r")+`}`, "")
	require.Equal(t, `ls\u200b\x1b]0; rm -rf /`, ToolPayloadPreview(brief))
	require.Equal(t, `list\U000e003b\r`, ToolPreview(brief))
	require.Equal(t, []Line{{LineCmd, "$ " + shown}, {LineOut, "red 👍\ufe0f"}}, ToolBody(ToolCall{Use: bash, Result: &colored}),
		"a result is not up for approval, so it is only stripped")
	require.Equal(t, `{"k":"a\u202eb"}`, ToolPayloadPreview(use(t, "agent.mcp_tool_use", "q", "{\"k\":\"a\u202eb\"}", "")))
	require.Equal(t, []Line{{LineOut, "{"}, {LineOut, `  "k": "` + unseenShown + `"`}, {LineOut, "}"}}, ToolBody(ToolCall{Use: mcp}))
	require.Equal(t, `db\u202e/q\u200b`, ToolDisplayName(mcp))

	plain := "日本語 🙂 עברית é\xff\n\t"
	require.Equal(t, plain, showHidden(plain))
	require.Equal(t, `a\x9bb`, showHidden("a\x9bb"))
}

func TestSanitizedAtTheSource(t *testing.T) {
	f := fold(
		mk(t, "span.model_request_start", 100, ""),
		mk(t, "agent.message", 200, `"content":[{"type":"text","text":`+js("hi\x1b[2J there")+`}]`),
		mk(t, "agent.tool_use", 300, `"evaluated_permission":"ask","name":"agent_bash","input":{"command":`+js("echo a\r\nb")+`,"description":`+js("say\xc2\x9b hi")+`}`),
	)
	turn, call := f.Nodes()[0], f.Nodes()[0].Blocks[1].Calls[0]
	require.Equal(t, "hi there", EventText(turn.Blocks[0].Event))
	require.Equal(t, `db\x1b]0;t\a/q`, ToolDisplayName(use(t, "agent.mcp_tool_use", "q", `{}`, `"mcp_server_name":`+js("db\x1b]0;t\x07"))))
	require.Equal(t, `say\u009b hi`, ToolPreview(call.Use))
	require.Equal(t, `echo a\r b`, ToolPayloadPreview(call.Use))
	require.Equal(t, []Line{{LineCmd, `$ echo a\r`}, {LineCmd, "b"}}, ToolBody(call))
	for _, l := range ToolBody(ToolCall{Use: use(t, "agent.mcp_tool_use", "q", `{"k":`+js("v\x7f\xc2\x90")+`}`, "")}) {
		require.False(t, strings.ContainsAny(l.Text, "\x7f\xc2\x90"), "%q", l.Text)
	}
}
