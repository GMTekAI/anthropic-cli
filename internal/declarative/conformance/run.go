package conformance

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
)

// Adapter is a tool under test.
type Adapter interface {
	// Name is how cases refer to the tool in `skip` and `only`.
	Name() string
	// Supports reports whether the tool manages a kind at all.
	Supports(kind string) bool
	// Start begins one case in a fresh working directory and state.
	Start(t *testing.T) Session
}

// Session is one case's worth of a tool: a working directory, its state, and
// the operations the runner drives. Each step calls Render once, then Plan;
// unless the step expects an error, it then calls Apply and Plan again on the
// same files.
type Session interface {
	// Render writes desired as the tool's native files, replacing whatever
	// the previous step wrote. Ref values are the adapter's to translate.
	Render(desired Desired) error
	// Plan reports what the tool would do, by logical name. A plan the tool
	// refuses to apply (a blocked resource, a wrong-target guard) is an
	// error, not a Plan.
	Plan(ctx context.Context, flags Flags) (Plan, error)
	// Apply converges. It may re-plan internally.
	Apply(ctx context.Context, flags Flags) error
	// IDs maps every logical name the tool currently tracks to its remote id.
	IDs() map[string]string
	// Destroy removes everything the session created, however the tool
	// does that.
	Destroy(ctx context.Context) error
}

// Remote is the runner's own line to the API, for out-of-band edits and for
// reading back what a tool actually did. It must not share the tool's client,
// so a bug in that client cannot also hide itself from the checks.
type Remote interface {
	// Get returns the object as decoded JSON. Once the object is deleted, the
	// error wraps ErrNotFound. An archived object comes back with a non-nil
	// archived_at, which is how teardown tells archived from live.
	Get(ctx context.Context, kind, id string) (map[string]any, error)
	// Update edits the object in place: a merge-style body and, for kinds
	// whose content is files, a new set of them.
	Update(ctx context.Context, kind, id string, body map[string]any, files map[string]string) error
	// Destroy archives or deletes, whichever the kind supports.
	Destroy(ctx context.Context, kind, id string) error
}

// ErrNotFound is what Remote.Get wraps for a resource that no longer exists.
var ErrNotFound = errors.New("not found")

// Options tune a run.
type Options struct {
	// Parallel runs cases as parallel subtests. Every name carries {run}, and
	// by convention each case gives its names a distinct suffix, so rate
	// limits are the only reason not to.
	Parallel bool
	// KeepGoing reports a failed expectation and carries on, through the rest
	// of the step and the steps after it, instead of ending the case. An error
	// from the tool itself (render, plan, apply) still ends the case.
	KeepGoing bool
}

// Run drives each case through the adapter as a subtest named after the case.
// A case is skipped when its skip or only rules exclude the adapter, or when it
// declares a kind the adapter does not manage. A case that runs is always torn
// down, even when a step fails.
func Run(t *testing.T, cases []*Case, tool Adapter, api Remote, opts Options) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if opts.Parallel {
				t.Parallel()
			}
			if why, skip := c.skipFor(tool); skip {
				t.Skip(why)
			}
			(&runner{t: t, c: c, tool: tool, api: api, opts: opts}).run()
		})
	}
}

func (c *Case) skipFor(tool Adapter) (string, bool) {
	if why, ok := c.Skip[tool.Name()]; ok {
		return fmt.Sprintf("skipped for %s: %s", tool.Name(), why), true
	}
	if len(c.Only) > 0 && !slices.Contains(c.Only, tool.Name()) {
		return fmt.Sprintf("only for %s", strings.Join(c.Only, ", ")), true
	}
	for _, k := range c.Kinds() {
		if !tool.Supports(k) {
			return fmt.Sprintf("%s does not manage %s resources", tool.Name(), k), true
		}
	}
	return "", false
}

type runner struct {
	t       *testing.T
	c       *Case
	tool    Adapter
	api     Remote
	opts    Options
	session Session
	// state is the desired state as of the current step.
	state Desired
	// kinds is the kind of every logical name ever declared.
	kinds map[string]string
	// ids is every id ever assigned to a name, so teardown can check
	// resources a step removed along the way.
	ids map[string]string
}

func (r *runner) run() {
	ctx := context.Background()
	r.session = r.tool.Start(r.t)
	r.state = Desired{}
	r.kinds = map[string]string{}
	r.ids = map[string]string{}
	for n, res := range r.c.Resources {
		r.state[n] = res
		r.kinds[n] = res.Kind
	}
	defer r.teardown(ctx)

	for i, step := range r.c.Steps {
		if !r.runStep(ctx, i, step) {
			return
		}
	}
}

// runStep runs one step and reports whether the case should go on to the next.
func (r *runner) runStep(ctx context.Context, i int, step Step) bool {
	t := r.t
	label := fmt.Sprintf("step %d", i+1)
	failAndContinue := func(format string, args ...any) (goOn bool) {
		t.Helper()
		t.Errorf(label+": "+format, args...)
		return r.opts.KeepGoing
	}

	next, err := nextDesired(r.state, step.Change)
	if err != nil {
		t.Fatalf("%s: %v", label, err)
	}
	r.state = next
	for n, res := range r.state {
		r.kinds[n] = res.Kind
	}

	for _, name := range sortedKeys(step.Remote) {
		if err := r.editRemote(ctx, name, step.Remote[name]); err != nil {
			t.Fatalf("%s: remote.%s: %v", label, name, err)
		}
	}

	if err := r.session.Render(r.state); err != nil {
		t.Fatalf("%s: render: %v", label, err)
	}
	plan, err := r.session.Plan(ctx, step.Flags)

	if step.Expect.Error != nil {
		return r.expectFailure(ctx, step, plan, err, failAndContinue)
	}
	if err != nil {
		t.Fatalf("%s: plan: %v", label, err)
	}

	if step.Expect.Plan != nil {
		if diff := comparePlan(plan, step.Expect.Plan, r.state); diff != "" {
			if !failAndContinue("plan differs:\n%s", diff) {
				return false
			}
		}
	}

	if err := r.session.Apply(ctx, step.Flags); err != nil {
		t.Fatalf("%s: apply: %v", label, err)
	}
	r.recordIDs()

	// The property every case exists to protect: having applied, there is
	// nothing left to do.
	again, err := r.session.Plan(ctx, step.Flags)
	if err != nil {
		t.Fatalf("%s: re-plan after apply: %v", label, err)
	}
	if pending := pendingChanges(again); len(pending) > 0 {
		if !failAndContinue("re-plan after apply is not empty:\n%s", strings.Join(pending, "\n")) {
			return false
		}
	}

	failed := false
	for _, name := range sortedKeys(step.Expect.Remote) {
		for _, msg := range r.checkRemote(ctx, name, step.Expect.Remote[name]) {
			failAndContinue("remote.%s: %s", name, msg)
			failed = true
		}
	}
	return !failed || r.opts.KeepGoing
}

// expectFailure checks a step that must fail. The refusal may come from the
// plan or, when the plan accepts, from the apply; either one satisfies the
// expectation.
func (r *runner) expectFailure(ctx context.Context, step Step, plan Plan, err error, failAndContinue func(string, ...any) bool) bool {
	if err == nil {
		err = r.session.Apply(ctx, step.Flags)
		r.recordIDs()
	}
	if err == nil {
		return failAndContinue("want an error matching /%s/; plan and apply both succeeded (plan: %s)", step.Expect.Error, plan)
	}
	if !step.Expect.Error.MatchString(err.Error()) {
		return failAndContinue("want an error matching /%s/, got:\n%v", step.Expect.Error, err)
	}
	return true
}

func (r *runner) recordIDs() {
	for name, id := range r.session.IDs() {
		if id != "" {
			r.ids[name] = id
		}
	}
}

// editRemote makes the out-of-band edit a step's `remote` key asks for.
func (r *runner) editRemote(ctx context.Context, name string, act RemoteAction) error {
	id, ok := r.id(name)
	if !ok {
		// A resource this step removed from the files is no longer tracked
		// but still exists on the server.
		id, ok = r.ids[name]
	}
	if !ok {
		return fmt.Errorf("%s has not been created yet", name)
	}
	kind := r.kinds[name]
	switch act.Verb {
	case "archive", "delete":
		return r.api.Destroy(ctx, kind, id)
	case "patch":
		return r.api.Update(ctx, kind, id, act.Body, act.Files)
	}
	return fmt.Errorf("unknown action %q", act.Verb)
}

// comparePlan renders a mismatch as a two-column diff, or "" when the plan is
// as expected. Every resource in the desired state that the expectation does
// not mention must be a noop.
func comparePlan(got, want Plan, state Desired) string {
	names := map[string]bool{}
	for n := range want {
		names[n] = true
	}
	for n := range state {
		names[n] = true
	}
	for n := range got {
		names[n] = true
	}
	var lines []string
	for _, n := range sortedKeys(names) {
		w, g := plannedOrNoop(want, n), plannedOrNoop(got, n)
		if g.Action == w.Action && (len(w.Fields) == 0 || slices.Equal(g.Fields, w.Fields)) {
			continue
		}
		lines = append(lines, fmt.Sprintf("  %-24s want %-22s got %s", n, w, g))
	}
	return strings.Join(lines, "\n")
}

// plannedOrNoop is a resource's line in a plan, where an unlisted name is a noop.
func plannedOrNoop(plan Plan, name string) Planned {
	if p, ok := plan[name]; ok {
		return p
	}
	return Planned{Action: ActionNoop}
}

// pendingChanges lists, one formatted line per resource in name order, every
// entry in plan whose action is not ActionNoop.
func pendingChanges(plan Plan) []string {
	var out []string
	for _, n := range sortedKeys(plan) {
		if plan[n].Action != ActionNoop {
			out = append(out, fmt.Sprintf("  %-24s %s", n, plan[n]))
		}
	}
	return out
}

// checkRemote reads the named resource back and returns one message per
// failed path.
func (r *runner) checkRemote(ctx context.Context, name string, want map[string]any) []string {
	id, ok := r.id(name)
	if !ok {
		return []string{"not tracked by the tool after apply"}
	}
	obj, err := r.api.Get(ctx, r.kinds[name], id)
	if err != nil {
		return []string{err.Error()}
	}
	var msgs []string
	for _, path := range sortedKeys(want) {
		got, present := lookup(obj, path)
		if err := match(ctx, got, present, want[path], r); err != nil {
			msgs = append(msgs, fmt.Sprintf("%s: %v", path, err))
		}
	}
	return msgs
}

// id and version make the runner a resolver for $id / $version matchers.
// version fetches the object and reports the first version field it carries,
// `version` or `latest_version_id`.
func (r *runner) id(name string) (string, bool) {
	id, ok := r.session.IDs()[name]
	return id, ok
}

func (r *runner) version(ctx context.Context, name string) (string, error) {
	id, ok := r.id(name)
	if !ok {
		return "", fmt.Errorf("%s is not tracked", name)
	}
	obj, err := r.api.Get(ctx, r.kinds[name], id)
	if err != nil {
		return "", err
	}
	for _, k := range []string{"version", "latest_version_id"} {
		if v, ok := obj[k]; ok && v != nil {
			return fmt.Sprint(v), nil
		}
	}
	return "", fmt.Errorf("%s has no version field", name)
}

func (r *runner) teardown(ctx context.Context) {
	t := r.t
	if err := r.session.Destroy(ctx); err != nil {
		t.Errorf("teardown: %v (resources named *%s* may need removing by hand)", err, r.ids)
		return
	}
	for name, id := range r.ids {
		obj, err := r.api.Get(ctx, r.kinds[name], id)
		switch {
		case errors.Is(err, ErrNotFound):
			// Deleted outright, which is as clean as archived.
		case err != nil:
			t.Errorf("teardown: reading back %s (%s): %v", name, id, err)
		case obj["archived_at"] == nil:
			t.Errorf("teardown: %s (%s %s) still exists and is not archived", name, r.kinds[name], id)
		}
	}
}

func sortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}
