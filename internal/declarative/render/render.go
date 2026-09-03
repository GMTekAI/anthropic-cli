// Package render turns a declarative plan into terminal output. It is separate
// from the reconciler so the engine has no opinion about presentation, and so
// the plan's public surface is the only thing a formatter can reach.
//
// The output has two phases with the same table shape: a preview of what will
// happen, and — once approved — a live account of what did. The preview is one
// line per resource so it can be taken in at a glance, and expands in place to
// show the field-by-field detail on request.
package render

import (
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/anthropics/anthropic-cli/internal/declarative/core"
	"github.com/charmbracelet/lipgloss"
)

// Renderer writes everything `apply` shows a person: the first-run notice, the
// untracked-resource offer, the plan and its prompt, then the apply as it runs.
// Out is required; the other fields are optional.
type Renderer struct {
	// Out receives everything the renderer prints.
	Out io.Writer
	// Color enables ANSI styling, and with it the escapes that hyperlinks (Link)
	// and in-place redraws (Rewind) rely on.
	Color bool
	// Verbose shows unchanged resources, full field values and whole prose
	// diffs rather than the edited stretch.
	Verbose bool
	// Expanded prints each resource's field-level detail beneath its row.
	Expanded bool
	// Viewport, when set, wraps long values to the terminal's width and lets
	// the preview be redrawn in place: see Rewind.
	Viewport Viewport
	// Link, when set, returns a URL for a resource ID (or ""), and IDs are
	// printed as terminal hyperlinks to it. Only honoured with Color, since a
	// terminal that takes no colour will not take the escape either.
	Link func(kind core.Kind, id string) string

	cols    columns
	counter *rowCounter
}

type styleFunc = func(...string) string

// The palette is the CLI's own: the accent that marks each phase is the brand
// orange, and create/update/remove use the same green, amber and red as the
// rest of the tool's diagnostics. lipgloss degrades these to the terminal's
// profile, and Color gates them so a piped plan stays plain text.
var (
	accentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#D97757")).Bold(true).Render
	labelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Render
	createStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#4EBA65")).Render
	updateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFC107")).Render
	// destroyStyle is the red for anything going away: a removed resource or
	// field, and an error.
	destroyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B80")).Render
	dimStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Render
	boldStyle         = lipgloss.NewStyle().Bold(true).Render
	insertedWordStyle = lipgloss.NewStyle().Background(lipgloss.Color("#225C2B")).Foreground(lipgloss.Color("#E9FBEC")).Render
	deletedWordStyle  = lipgloss.NewStyle().Background(lipgloss.Color("#7A2936")).Foreground(lipgloss.Color("#FDECEF")).Strikethrough(true).Render
)

const (
	// valueWidth bounds a rendered field value before it gets elided. Wide
	// enough to read an ID or a short prompt, narrow enough that one field
	// stays one line.
	valueWidth = 96
	// maxNameWidth caps the name column, so one long key does not push every
	// status off the right of the screen.
	maxNameWidth = 56
	// indentStep is one level of nesting in field detail.
	indentStep = "    "
)

// RenderPlan writes the preview phase: header, warnings, one table row per
// resource — with its detail beneath when Expanded — a summary, and anything
// blocking the apply. It marks where it began so a prompt can Rewind and draw
// it again the other way.
func (r *Renderer) RenderPlan(plan *core.Plan) {
	r.markRewindPoint()
	r.measure(plan)

	note := DisplayPath(plan.LockfilePath)
	if !plan.LockfileExisted {
		note += " (new)"
	}
	r.heading("Preview", note)
	for _, w := range plan.Warnings {
		r.line(r.paint(updateStyle, "warning: ") + w)
	}
	r.blank()

	shown, afterDetail := 0, false
	for _, c := range plan.Changes {
		if !r.listed(c) {
			continue
		}
		if shown == 0 {
			r.columnHeader("Plan")
		}
		if afterDetail {
			// Set an expanded block off from the next row, not from the summary.
			r.blank()
		}
		r.planRow(c)
		afterDetail = r.Expanded && r.detail(c, indentStep)
		shown++
	}
	if shown == 0 {
		r.line(r.paint(dimStyle, "Everything is up to date."))
	}

	create, update, destroy, noop := plan.Counts()
	r.summary("to create", "to update", "to remove", create, update, destroy, noop)

	if blocked := plan.Blocked(); len(blocked) > 0 {
		r.blank()
		r.line(r.paint(destroyStyle, "This plan cannot be applied:"))
		for _, c := range blocked {
			r.line(r.paint(destroyStyle, "✗ ") + r.paint(boldStyle, c.Key))
			r.line(indentStep + c.Blocked.Error())
		}
	}
}

// planRow is one line: what will happen to the resource and, for an update,
// which fields it touches. An update shows its ID only when expanded, in place
// of the field hint. A blocked change trades its glyph for a ✗ and is
// explained beneath the summary.
func (r *Renderer) planRow(c *core.Change) {
	id := ""
	if c.Entry != nil {
		id = r.id(c.Kind, c.Entry.ID)
	}
	if c.Action == core.ActionNoop {
		r.row("", dimStyle, c.Key, "unchanged", dimStyle, id)
		return
	}
	glyph, style := actionGlyph(c.Action)
	glyphStyle := style
	if c.Blocked != nil {
		glyph, glyphStyle = "✗", destroyStyle
	}
	switch c.Action {
	case core.ActionCreate:
		r.row(glyph, glyphStyle, c.Key, "create", style, "")
	case core.ActionUpdate:
		r.row(glyph, glyphStyle, c.Key, "update", style, r.updateInfo(c, id))
	case core.ActionDestroy:
		r.row(glyph, glyphStyle, c.Key, c.Destroy().Verb, style, id)
	}
}

// actionGlyph is the mark and colour a change of this kind carries in both
// the preview and the apply.
func actionGlyph(a core.Action) (string, styleFunc) {
	switch a {
	case core.ActionCreate:
		return "+", createStyle
	case core.ActionUpdate:
		return "~", updateStyle
	case core.ActionDestroy:
		return "-", destroyStyle
	}
	return "", dimStyle
}

// updateInfo is what follows an update's status: the ID when expanded,
// otherwise a hint of the fields it touches, and a warning when it has drifted.
func (r *Renderer) updateInfo(c *core.Change, id string) string {
	var info []string
	if r.Expanded {
		info = append(info, id)
	} else if hint := r.diffHint(c.Diff); hint != "" {
		info = append(info, hint)
	}
	if c.Drift {
		info = append(info, r.paint(updateStyle, "! edited outside this config"))
	}
	return strings.Join(info, "  ")
}

// diffHint names the top-level fields an update touches, glyph-coded, so the
// table row says roughly what changed without the reader opening the details:
// `~system +tools -description`.
func (r *Renderer) diffHint(d *core.Diff) string {
	if d == nil || d.Kind != core.DiffObject {
		return ""
	}
	const most = 4
	names := slices.Sorted(maps.Keys(d.Fields))
	var parts []string
	for i, name := range names {
		if i == most {
			parts = append(parts, r.paint(dimStyle, fmt.Sprintf("+%d more", len(names)-most)))
			break
		}
		glyph, style := diffGlyph(d.Fields[name].Kind)
		parts = append(parts, r.paint(style, glyph+name))
	}
	return r.paint(dimStyle, "[") + strings.Join(parts, " ") + r.paint(dimStyle, "]")
}

// summary prints the resource counts on one line, non-zero ones only:
// `Resources  + 1 to create · ~ 2 to update · 3 unchanged`.
func (r *Renderer) summary(createLabel, updateLabel, destroyLabel string, create, update, destroy, noop int) {
	var parts []string
	item := func(n int, glyph string, style styleFunc, label string) {
		if n > 0 {
			parts = append(parts, r.paint(style, fmt.Sprintf("%s %d %s", glyph, n, label)))
		}
	}
	item(create, "+", createStyle, createLabel)
	item(update, "~", updateStyle, updateLabel)
	item(destroy, "-", destroyStyle, destroyLabel)
	if noop > 0 {
		parts = append(parts, r.paint(dimStyle, fmt.Sprintf("%d unchanged", noop)))
	}
	if len(parts) == 0 {
		parts = append(parts, r.paint(dimStyle, "none"))
	}
	r.blank()
	r.line(r.paint(labelStyle, "Resources") + "  " + strings.Join(parts, r.paint(dimStyle, " · ")))
}

// BeginApply opens the second phase: its heading, noting the lockfile path,
// and the table's column header.
func (r *Renderer) BeginApply(plan *core.Plan) {
	r.blank()
	r.heading("Apply", DisplayPath(plan.LockfilePath))
	r.blank()
	r.columnHeader("Status")
}

// Applied prints one resource's outcome as it lands. It has the signature of
// core.Applier.Report.
func (r *Renderer) Applied(applied core.Applied) {
	c := applied.Change
	id := r.id(c.Kind, applied.ID)
	if applied.Err != nil {
		r.row("✗", destroyStyle, c.Key, applied.Outcome, destroyStyle, id)
		if hint := core.Hint(applied.Err); hint != "" {
			r.wrapLine(indentStep+r.paint(updateStyle, "hint: "), indentStep+indentStep, hint)
		}
		return
	}
	if c.Action == core.ActionNoop {
		if r.Verbose {
			r.row("", dimStyle, c.Key, applied.Outcome, dimStyle, id)
		}
		return
	}
	// The glyph keeps the plan's colour for what kind of change it was; the
	// status word says whether it worked, so every success reads green and
	// only the ✗ row above is red.
	glyph, style := actionGlyph(c.Action)
	r.row(glyph, style, c.Key, applied.Outcome, createStyle, id)
}

// RenderResult closes the second phase with what was actually done. err is
// the apply's error, if it stopped early; the caller still reports it in full.
func (r *Renderer) RenderResult(res *core.Result, lockfilePath string, err error) {
	r.summary("created", "updated", "removed", res.Created, res.Updated, res.Destroyed, res.Unchanged)
	r.blank()
	if err != nil {
		r.line(r.paint(destroyStyle, "Stopped at the first failure; everything before it is recorded in "+DisplayPath(lockfilePath)))
		return
	}
	r.line(r.paint(dimStyle, "State written to "+DisplayPath(lockfilePath)))
}

// columns is the table layout, sized once from the plan so both phases align.
type columns struct {
	name int
}

// listed reports whether the preview gives c a row: unchanged resources only
// when verbose.
func (r *Renderer) listed(c *core.Change) bool {
	return c.Action != core.ActionNoop || r.Verbose
}

// measure sizes the columns to the rows RenderPlan will show.
func (r *Renderer) measure(plan *core.Plan) {
	r.cols = columns{name: len("Name")}
	for _, c := range plan.Changes {
		if !r.listed(c) {
			continue
		}
		r.cols.name = max(r.cols.name, len(c.Key))
	}
	r.cols.name = min(r.cols.name, maxNameWidth)
}

// columnHeader labels the table. The change column gets a ± so the header
// sits flush with the rows beneath it rather than floating over the names.
func (r *Renderer) columnHeader(status string) {
	r.line(r.paint(labelStyle, fmt.Sprintf("± %s  %s", pad("Name", r.cols.name), status)))
}

// row prints one table line: glyph, name, status, and trailing info. There is
// no kind column: the name's directory nearly always says it, and the detail
// heading carries it when it does not.
func (r *Renderer) row(glyph string, glyphStyle styleFunc, name, status string, statusStyle styleFunc, info string) {
	if glyph == "" {
		glyph = " "
	}
	// Nine cells fit every plan status ("unchanged" is the longest); a longer
	// apply outcome such as "already gone" just pushes the info column right.
	s := fmt.Sprintf("%s %s  %s",
		r.paint(glyphStyle, glyph),
		pad(name, r.cols.name),
		r.paint(statusStyle, pad(status, 9)))
	if info != "" {
		s += "  " + info
	}
	r.line(strings.TrimRight(s, " "))
}

// heading opens a phase: a title in the accent colour and a dim note.
func (r *Renderer) heading(title, note string) {
	s := r.paint(accentStyle, title)
	if note != "" {
		s += "  " + r.paint(dimStyle, note)
	}
	r.line(s)
}

func (r *Renderer) paint(style styleFunc, s string) string {
	if !r.Color {
		return s
	}
	return style(s)
}

func (r *Renderer) line(s string) { fmt.Fprintln(r.out(), s) }

func (r *Renderer) blank() { fmt.Fprintln(r.out()) }

// id renders a resource ID dimmed and, with Color on and a URL from Link, as
// an OSC 8 hyperlink. Terminals without hyperlink support show the plain text.
func (r *Renderer) id(kind core.Kind, id string) string {
	text := r.paint(dimStyle, id)
	if !r.Color || r.Link == nil || id == "" {
		return text
	}
	url := r.Link(kind, id)
	if url == "" {
		return text
	}
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

func pad(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

// DisplayPath shows a path relative to the working directory when it is
// beneath it, which is nearly always and much shorter.
func DisplayPath(p string) string {
	wd, err := os.Getwd()
	if err != nil {
		return p
	}
	rel, err := filepath.Rel(wd, p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return p
	}
	if rel == "." {
		return "./"
	}
	return "./" + filepath.ToSlash(rel)
}
