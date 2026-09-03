package render

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/anthropics/anthropic-cli/internal/declarative/core"
)

// Notices and questions around the two phases: where a first apply will create
// things, what was found untracked, and whether to go ahead.

// OriginSummary is the origin a first apply will create its resources in, in
// words: how the run authenticates and the host, organization and workspace
// that reaches.
type OriginSummary struct {
	Credentials  string // "profile <name>", or how else the run authenticates
	Host         string
	Organization string // display form: name and/or id
	Workspace    string
}

// FirstRun tells the user, before anything is planned, that this directory
// has no state yet and where its resources are about to be created.
func (r *Renderer) FirstRun(lockfilePath string, c OriginSummary) {
	r.heading("First apply", DisplayPath(lockfilePath)+" does not exist yet and will be created")
	r.blank()
	r.line(r.paint(labelStyle, "Resources will be created with"))
	r.originRows(c)
	r.blank()
}

func (r *Renderer) originRows(c OriginSummary) {
	row := func(k, v string) {
		if v != "" {
			r.line("  " + r.paint(dimStyle, pad(k, 14)) + v)
		}
	}
	row("credentials", c.Credentials)
	row("host", c.Host)
	row("organization", c.Organization)
	row("workspace", c.Workspace)
}

// Untracked lists resources found on disk that the lockfile does not track,
// ahead of asking whether to include them.
func (r *Renderer) Untracked(root string, found []core.Found, firstRun bool) {
	note := "not in the lockfile"
	if firstRun {
		note = "nothing is tracked yet"
	}
	r.heading("Found untracked resources", DisplayPath(root)+" — "+note)
	r.blank()
	width := 0
	for _, f := range found {
		width = max(width, len(f.Key))
	}
	for _, f := range found {
		r.line(fmt.Sprintf("%s %s  %s", r.paint(createStyle, "+"), pad(f.Key, min(width, maxNameWidth)), r.paint(dimStyle, string(f.Kind))))
	}
}

// Confirm asks a yes/no question, defaulting to yes.
func (r *Renderer) Confirm(in io.Reader, question string) (bool, error) {
	r.blank()
	fmt.Fprint(r.out(), r.ask(question, "(Y)es / (n)o"))
	var line string
	// Fscanln reports a bare Enter as "unexpected newline", an error fmt does
	// not export, and closed input as io.EOF. Both take the default answer.
	_, err := fmt.Fscanln(in, &line)
	if err != nil && err.Error() != "unexpected newline" && !errors.Is(err, io.EOF) {
		return false, err
	}
	r.blank()
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "" || answer == "y" || answer == "yes", nil
}

// Prompt writes the confirmation question, without a newline. When hasDetails
// is set the reader is offered a third answer that toggles the expanded view.
func (r *Renderer) Prompt(hasDetails bool) {
	r.blank()
	opts := "(y)es / (n)o"
	if hasDetails && r.Expanded {
		opts += " / (d) hide details"
	} else if hasDetails {
		opts += " / (d)etails"
	}
	fmt.Fprint(r.out(), r.ask("Apply these changes?", opts))
}

// ask renders a question awaiting an answer: plain text, so it reads as a
// question rather than another section title, with the options dimmed.
func (r *Renderer) ask(question, options string) string {
	return question + " " + r.paint(dimStyle, options) + " "
}
