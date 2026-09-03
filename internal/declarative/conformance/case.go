package conformance

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

// SchemaVersion is the case format this package reads.
const SchemaVersion = 1

// Case is one scenario: an initial desired state and a sequence of steps that
// change it, disturb the server, and say what a correct tool does next.
type Case struct {
	// Path is the file the case was read from.
	Path string
	Name string
	// Skip maps an Adapter's Name to the reason the case does not apply to it.
	Skip map[string]string
	// Only, when non-empty, names the only adapters the case runs against.
	Only      []string
	Resources Desired
	Steps     []Step
}

// Kinds lists every kind the case declares, across all steps, so a tool that
// does not manage one of them can skip the case whole.
func (c *Case) Kinds() []string {
	seen := map[string]bool{}
	for _, r := range c.Resources {
		seen[r.Kind] = true
	}
	for _, s := range c.Steps {
		for _, p := range s.Change {
			if p != nil && p.Kind != "" {
				seen[p.Kind] = true
			}
		}
	}
	return slices.Sorted(maps.Keys(seen))
}

// Resource is one API object as a case declares it.
type Resource struct {
	Kind string
	// Body is the create request body. Values may contain Ref.
	Body map[string]any
	// Files is a skill's content, keyed by path relative to its root.
	Files map[string]string
}

// Desired is a full desired state, keyed by logical name.
type Desired map[string]Resource

// Names lists the resources in a stable order.
func (d Desired) Names() []string {
	return slices.Sorted(maps.Keys(d))
}

// Ref stands in a body for a reference to another resource in the case. An
// adapter renders it however its tool spells a reference in that slot.
type Ref struct {
	Name string
}

// Step is one round: edit the files, poke the server, plan, apply, check.
type Step struct {
	// Change is a merge patch per resource against the previous step's
	// desired state. A nil patch removes the resource.
	Change map[string]*Patch
	// Remote lists out-of-band edits made through the API before planning.
	Remote map[string]RemoteAction
	Flags  Flags
	Expect Expect
}

// Patch is one resource's entry under `change`. A nil *Patch, written as
// null in the case file, removes the resource.
type Patch struct {
	// Kind is required when the name is new, and must match otherwise.
	Kind  string
	Body  map[string]any
	Files map[string]any // string content, or nil to delete the file
}

// RemoteAction is an out-of-band edit made through Remote.
type RemoteAction struct {
	// Verb is "archive", "delete" or "patch". Body and Files are the patch's
	// merge-style update and new file set, and are unused otherwise.
	Verb  string
	Body  map[string]any
	Files map[string]string
}

// Flags are the tool options a step runs with.
type Flags struct {
	Force bool
	Prune bool
}

// Expect is what a step asserts.
type Expect struct {
	// Plan maps logical names to the action a correct plan takes. Nil skips
	// the check; otherwise unlisted names must be ActionNoop.
	Plan Plan
	// Error, when set, is a pattern the plan or apply must fail with.
	Error *regexp.Regexp
	// Remote maps logical names to dotted-path assertions on the object as
	// the API returns it after the step applies.
	Remote map[string]map[string]any
}

// Action is the tool-neutral vocabulary for what a plan does to a resource.
type Action string

const (
	ActionCreate Action = "create"
	ActionNoop   Action = "noop"
	ActionUpdate Action = "update"
	// ActionReplace is a new object created in place of one the tool can no
	// longer use.
	ActionReplace Action = "replace"
	ActionDestroy Action = "destroy"
)

// Planned is one resource's line in a plan.
type Planned struct {
	Action Action
	// Fields are the top-level body fields an Update touches, sorted. Empty
	// in an expectation means "don't check which".
	Fields []string
}

// String renders p for a plan mismatch report: the bare action, or
// update[field ...] when Fields is set.
func (p Planned) String() string {
	if len(p.Fields) == 0 {
		return string(p.Action)
	}
	return fmt.Sprintf("%s%v", p.Action, p.Fields)
}

// Plan is what a tool would do, keyed by logical name.
type Plan map[string]Planned

// validate catches what would otherwise surface as a confusing failure three
// API calls into a run.
func (c *Case) validate(runID string) error {
	if len(c.Steps) == 0 {
		return fmt.Errorf("no steps")
	}
	state := c.Resources
	for i, s := range c.Steps {
		if i == 0 && len(s.Change) > 0 {
			return fmt.Errorf("steps[0]: the first step plans `resources` as given; put changes in a later step")
		}
		next, err := nextDesired(state, s.Change)
		if err != nil {
			return fmt.Errorf("steps[%d]: %w", i, err)
		}
		state = next
		for name, r := range state {
			if !mentionsRun(r, runID) {
				return fmt.Errorf("steps[%d]: %s: name must contain {run} so parallel runs in one organization cannot collide", i, name)
			}
			if err := checkRefs(name, r.Body, state); err != nil {
				return fmt.Errorf("steps[%d]: %w", i, err)
			}
		}
		for name := range s.Remote {
			// A name gone from this step's state still passes if `resources`
			// declared it; the runner then checks its id.
			_, now := state[name]
			_, declared := c.Resources[name]
			if !now && !declared {
				return fmt.Errorf("steps[%d]: remote.%s: no such resource", i, name)
			}
		}
	}
	return nil
}

// mentionsRun reports whether the name the API will see for r contains runID:
// its body's name, else its display_name, else, for a skill, its SKILL.md.
func mentionsRun(r Resource, runID string) bool {
	if name, ok := r.Body["name"].(string); ok {
		return strings.Contains(name, runID)
	}
	if displayName, ok := r.Body["display_name"].(string); ok {
		return strings.Contains(displayName, runID)
	}
	if md, ok := r.Files["SKILL.md"]; ok {
		return strings.Contains(md, runID)
	}
	return false
}

// checkRefs reports the first Ref under v that names no resource in state,
// or that names owner itself.
func checkRefs(owner string, v any, state Desired) error {
	switch t := v.(type) {
	case Ref:
		if _, ok := state[t.Name]; !ok {
			return fmt.Errorf("%s: $ref %q names no resource in this step", owner, t.Name)
		}
		if t.Name == owner {
			return fmt.Errorf("%s: $ref to itself", owner)
		}
	case map[string]any:
		for _, val := range t {
			if err := checkRefs(owner, val, state); err != nil {
				return err
			}
		}
	case []any:
		for _, val := range t {
			if err := checkRefs(owner, val, state); err != nil {
				return err
			}
		}
	}
	return nil
}
