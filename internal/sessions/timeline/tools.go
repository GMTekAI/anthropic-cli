package timeline

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/tidwall/gjson"
)

// previewMaxRunes caps a one-line tool preview.
const previewMaxRunes = 100

// LineKind says how a renderer should style a Line of a tool body.
type LineKind int

const (
	LineMeta LineKind = iota // dim header: a path, or "@@" between edits
	LineCmd                  // the call's one-line input: "$ cmd", "pattern  in path", url
	LineOut                  // payload: result text, written content, diff context, input JSON
	LineAdd
	LineDel
	LineErr   // result text of a failed call
	LineNote  // "(non-text content)", "… N more lines"
	LineAllow // the user's verdict: "Allowed"
	LineDeny  // "Denied — msg"
)

// Line is one styled line of a tool body.
type Line struct {
	Kind LineKind
	Text string
}

// ToolKind maps a tool use to the built-in whose layout applies; "" means no
// built-in layout applies. MCP servers and custom tools choose their own names,
// so only agent.tool_use events map to a built-in.
func ToolKind(ev Event) string {
	if !isBuiltin(ev) {
		return ""
	}
	switch base := baseName(ev.Name); base {
	case "bash", "read", "write", "glob", "grep", "web_search", "web_fetch":
		return base
	case "edit", "text_editor", "str_replace", "str_replace_editor", "multi_edit":
		return "edit"
	}
	return ""
}

// isBuiltin reports whether ev calls one of the agent's own tools with input a
// built-in layout can show. Parsers disagree on which of a repeated key wins,
// so a field picked from such input may not be the one the tool reads.
func isBuiltin(ev Event) bool {
	return ev.Type == "agent.tool_use" && !repeatsKey(gjson.Parse(ev.JSON.Input.Raw()))
}

func repeatsKey(v gjson.Result) bool {
	seen, repeats := map[string]bool{}, false
	v.ForEach(func(key, value gjson.Result) bool {
		repeats = v.IsObject() && seen[key.String()] || (value.IsObject() || value.IsArray()) && repeatsKey(value)
		seen[key.String()] = true
		return !repeats
	})
	return repeats
}

// baseName strips the one runtime prefix built-ins arrive under.
func baseName(name string) string {
	for _, prefix := range []string{"agent_", "mcp_", "computer_"} {
		if rest, ok := strings.CutPrefix(name, prefix); ok {
			return rest
		}
	}
	return name
}

// ToolDisplayName is the name to show for a tool use, qualified by its MCP
// server when it has one.
func ToolDisplayName(ev Event) string {
	name := baseName(ev.Name)
	if ev.Type == "agent.mcp_tool_use" && ev.MCPServerName != "" {
		name = ev.MCPServerName + "/" + name
	}
	return Name(name)
}

// Name is a name an agent, server or tool chose for itself: whole, on one
// line, with hidden characters shown.
func Name(s string) string { return strings.Join(strings.Fields(Sanitize(showHidden(s))), " ") }

var (
	previewKeys = []string{"description", "command", "script", "file_path", "path", "query", "url", "pattern"}
	payloadKeys = []string{"command", "script", "code", "file_path", "path", "url", "query", "pattern", "description"}
)

// ToolPreview is the call's one identifying argument on a single line: the
// first string-valued key in a fixed precedence, whitespace collapsed, capped
// at previewMaxRunes.
func ToolPreview(ev Event) string { return preview(ev, previewKeys) }

// ToolPayloadPreview prefers what will actually execute over the model's
// own description of it, for places where the user is asked to approve it.
func ToolPayloadPreview(ev Event) string { return preview(ev, payloadKeys) }

// MCP and custom tools give their fields their own meanings, so their whole
// input is shown instead of one field.
func preview(ev Event, keys []string) string {
	if !isBuiltin(ev) {
		return oneLine(ev.JSON.Input.Raw())
	}
	input := gjson.Parse(ev.JSON.Input.Raw())
	for _, key := range keys {
		if value := input.Get(key); value.Type == gjson.String {
			return oneLine(value.String())
		}
	}
	return ""
}

func oneLine(s string) string {
	s = Name(s)
	if runes := []rune(s); len(runes) > previewMaxRunes {
		return string(runes[:previewMaxRunes]) + "…"
	}
	return s
}

// ToolBody is the expanded view of a call: the user's verdict if any, the
// input in the tool's own idiom, then the result. Write and edit results are
// one-line acks, so they only surface on error.
func ToolBody(call ToolCall) []Line {
	lines := toolBody(call)
	for i := range lines {
		lines[i].Text = Sanitize(lines[i].Text)
	}
	return lines
}

func toolBody(call ToolCall) []Line {
	var out []Line
	if confirmation := call.Confirmation; confirmation != nil {
		if line, ok := confirmationLine(*confirmation); ok {
			out = append(out, line)
		}
	}
	input, resultOnlyOnError := toolInput(call.Use, call.Lifecycle() == AwaitingApproval)
	out = append(out, input...)
	for i := range out {
		out[i].Text = showHidden(out[i].Text)
	}
	if call.Result == nil || (resultOnlyOnError && !call.Result.IsError) {
		return out
	}
	text := rawEventText(*call.Result)
	if raw := strings.TrimSpace(call.Result.JSON.Content.Raw()); text == "" && raw != "" && raw != "[]" && raw != "null" {
		return append(out, Line{LineNote, "(non-text content)"})
	}
	resultKind := LineOut
	if call.Result.IsError {
		resultKind = LineErr
	}
	return appendText(out, resultKind, text)
}

// toolInput is a call's input in the tool's own idiom, and whether only a failed
// result is worth showing. A built-in lists the fields its idiom left out only
// while it awaits approval, when the user's verdict depends on them.
func toolInput(use Event, awaiting bool) (out []Line, resultOnlyOnError bool) {
	raw := use.JSON.Input.Raw()
	if !isBuiltin(use) {
		return wholeInput(raw), false
	}
	input := newFields(gjson.Parse(raw))
	// A string description counts as shown: it is the model's note about the
	// call, which the tool does not act on.
	input.first("description")

	switch kind := ToolKind(use); kind {
	case "bash":
		out = appendText(out, LineCmd, "$ "+input.first("command"))
	case "glob":
		out = append(out, Line{LineCmd, cmp.Or(input.first("pattern"), "(none)")})
	case "grep":
		search := cmp.Or(input.first("pattern"), "(none)")
		if path := input.first("file_path", "path"); path != "" {
			search += "  in " + path
		}
		out = append(out, Line{LineCmd, search})
	case "web_search":
		out = append(out, Line{LineCmd, `"` + cmp.Or(input.first("query"), "(none)") + `"`})
	case "web_fetch":
		out = append(out, Line{LineCmd, cmp.Or(input.first("url"), "(none)")})
		if prompt := input.first("prompt"); prompt != "" {
			out = append(out, Line{LineCmd, "↳ " + prompt})
		}
	case "read", "write", "edit":
		// text_editor is command-discriminated (view / create / str_replace),
		// so route by the payload actually present rather than the name.
		out = append(out, Line{LineMeta, cmp.Or(input.first("file_path", "path"), "(no path)")})
		if kind == "read" {
			break
		}
		if fileText := input.first("content", "file_text"); kind == "write" || fileText != "" {
			out = appendText(out, LineOut, cmp.Or(fileText, "(empty)"))
			if fileText != "" && textLines(fileText) == nil {
				input.unshow("content", "file_text")
			}
			resultOnlyOnError = true
		} else if diff := editDiff(input); diff != nil {
			out = append(out, diff...)
			resultOnlyOnError = true
		}
	default:
		// A built-in call with a script or code field (such as repl or
		// code_execution) shows that source like a command. Any other shows
		// its full input as JSON.
		if code := input.first("script", "code"); code != "" {
			out = appendText(out, LineCmd, code)
			break
		}
		return wholeInput(raw), false
	}
	if awaiting {
		out = appendText(out, LineOut, strings.Join(input.rest(), "\n"))
	}
	return out, resultOnlyOnError
}

// wholeInput is raw as indented JSON, or as it arrived if it can't be indented.
func wholeInput(raw string) []Line {
	var pretty bytes.Buffer
	if json.Indent(&pretty, []byte(raw), "", "  ") != nil {
		return appendText(nil, LineOut, raw)
	}
	return appendText(nil, LineOut, pretty.String())
}

// fields picks the input fields a layout shows, remembering them so that the
// rest can be shown after it.
type fields struct {
	value gjson.Result
	shown map[string]bool
}

func newFields(value gjson.Result) fields { return fields{value, map[string]bool{}} }

// first is the first of keys holding a non-empty string. An empty string
// passed over has nothing to show, so it counts as shown.
func (f fields) first(keys ...string) string {
	for _, key := range keys {
		if value := f.value.Get(key); value.Type == gjson.String {
			f.shown[key] = true
			if value.String() != "" {
				return value.String()
			}
		}
	}
	return ""
}

// unshow returns keys whose text a layout could not render to rest.
func (f fields) unshow(keys ...string) {
	for _, key := range keys {
		delete(f.shown, key)
	}
}

// rest is each field not yet shown, as its raw `"key": value`.
func (f fields) rest() []string {
	var out []string
	f.value.ForEach(func(key, value gjson.Result) bool {
		if !f.shown[key.String()] {
			out = append(out, key.Raw+": "+value.Raw)
		}
		return true
	})
	return out
}

// confirmationLine is the user's verdict on a tool_confirmation; false when
// it carries neither allow nor deny.
func confirmationLine(ev Event) (Line, bool) {
	switch ev.AsUserToolConfirmation().Result {
	case "allow":
		return Line{LineAllow, "Allowed"}, true
	case "deny":
		if ev.DenyMessage != "" {
			return Line{LineDeny, "Denied — " + ev.DenyMessage}, true
		}
		return Line{LineDeny, "Denied"}, true
	}
	return Line{}, false
}

// editDiff renders old_str/new_str (either spelling, or multi_edit's edits[])
// as -/+ lines, keeping shared leading/trailing lines as context. nil when
// the input carries no edit payload.
func editDiff(input fields) []Line {
	edits := []fields{input}
	list := input.value.Get("edits")
	if list.IsArray() {
		edits = nil
		for _, edit := range list.Array() {
			edits = append(edits, newFields(edit))
		}
	}
	var out []Line
	for _, edit := range edits {
		oldText, newText := edit.first("old_str", "old_string"), edit.first("new_str", "new_string")
		if oldText == "" && newText == "" {
			continue
		}
		// textLines drops trailing newlines, so a hunk can't show a change to them.
		if trailingNewlines(oldText) != trailingNewlines(newText) {
			edit.unshow("old_str", "old_string", "new_str", "new_string")
		}
		if out != nil {
			out = append(out, Line{LineMeta, "@@"})
		}
		out = append(out, hunk(textLines(oldText), textLines(newText))...)
	}
	if list.IsArray() && !slices.ContainsFunc(edits, func(edit fields) bool { return len(edit.rest()) > 0 }) {
		input.shown["edits"] = true
	}
	return out
}

// hunk diffs two texts by trimming their common head and tail: what remains
// in between is one deletion followed by one addition.
func hunk(oldLines, newLines []string) []Line {
	head := 0
	for head < len(oldLines) && head < len(newLines) && oldLines[head] == newLines[head] {
		head++
	}
	tail := 0
	for tail < len(oldLines)-head && tail < len(newLines)-head &&
		oldLines[len(oldLines)-1-tail] == newLines[len(newLines)-1-tail] {
		tail++
	}
	var out []Line
	out = appendPrefixed(out, LineOut, " ", oldLines[:head])
	out = appendPrefixed(out, LineDel, "-", oldLines[head:len(oldLines)-tail])
	out = appendPrefixed(out, LineAdd, "+", newLines[head:len(newLines)-tail])
	return appendPrefixed(out, LineOut, " ", oldLines[len(oldLines)-tail:])
}

// appendText appends text to out as one Line of kind per line.
func appendText(out []Line, kind LineKind, text string) []Line {
	return appendPrefixed(out, kind, "", textLines(text))
}

func appendPrefixed(out []Line, kind LineKind, prefix string, texts []string) []Line {
	for _, text := range texts {
		out = append(out, Line{kind, prefix + text})
	}
	return out
}

// textLines splits without the phantom empty line a trailing newline or an
// empty payload would otherwise contribute.
func textLines(text string) []string {
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func trailingNewlines(s string) int { return len(s) - len(strings.TrimRight(s, "\n")) }

// Dense truncation caps for tool bodies shown inline in the transcript, matching the web viewer's.
const (
	denseMaxLines = 12
	denseMaxChars = 2000
)

// TruncateDense keeps the head of a body within the dense caps and appends a
// note saying what it dropped. denseMaxChars counts runes, newlines included;
// the line that crosses it is cut to the remaining budget on a rune boundary.
func TruncateDense(lines []Line) []Line {
	var kept []Line
	chars, cut := 0, false
	for _, line := range lines {
		if len(kept) == denseMaxLines || chars >= denseMaxChars {
			cut = true
			break
		}
		runes := utf8.RuneCountInString(line.Text)
		if chars+runes > denseMaxChars {
			line.Text, cut = string([]rune(line.Text)[:denseMaxChars-chars]), true
		}
		chars += runes + 1
		kept = append(kept, line)
	}
	if !cut {
		return lines
	}
	return append(kept, Line{LineNote, fmt.Sprintf("… %d more lines · %d of %d shown", len(lines)-len(kept), len(kept), len(lines))})
}
