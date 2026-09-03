// Package acctest drives `ant apply` lifecycles against a real API.
//
// A test is a list of Steps — edit files, plan, apply, check — and Run holds
// every step to the same standard: after it applies, a fresh plan must find
// nothing to do and a fresh apply must write nothing. When the test ends,
// everything it created is pruned and each ID is confirmed gone or archived.
//
// These tests create and destroy real resources, so they only run when
// ANT_ACC is set; see SkipUnlessEnabled. This package must not import the
// claude package (it takes a Registry and Client from the caller) so that
// claude's own tests can import it without a cycle.
package acctest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-cli/internal/declarative/core"
	"github.com/anthropics/anthropic-cli/internal/declarative/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SkipUnlessEnabled skips t unless acceptance tests are switched on. They are
// off by default because they need credentials and leave (archived) resources
// behind.
func SkipUnlessEnabled(t *testing.T) {
	t.Helper()
	if os.Getenv("ANT_ACC") == "" {
		t.Skip("acceptance test: set ANT_ACC=1 and credentials for a scratch organization")
	}
}

// Step is one round of "change something, plan, apply, check". Every field is
// optional; a zero Step re-plans the tree as it stands and requires that to
// be a no-op.
type Step struct {
	// Name is the subtest name. It defaults to "step N", counting from 1.
	Name string

	// Write creates or overwrites files (slash paths, relative to Fixture.Root)
	// before planning. "{{name}}" in content is replaced with
	// Fixture.UniqueName, for fields the server requires to be unique.
	Write map[string]string
	// Remove deletes files or directories before planning.
	Remove []string
	// BeforePlan runs after Remove and Write, before planning: to change
	// server state behind the reconciler's back through Fixture.Unrecorded,
	// or to write files whose content needs IDs an earlier step minted.
	BeforePlan func(t *testing.T, fx *Fixture)

	// Prune and Force set the core.Planner fields of the same names, as
	// --prune and --force do.
	Prune bool
	Force bool

	// ExpectActions maps resource key → the action the plan must choose.
	// Keys are lockfile keys: the path relative to Root with a leading "./",
	// such as "./agents/a.md", or the directory for a skill, "./skills/s".
	// Keys not listed are not checked.
	ExpectActions map[string]core.Action
	// ExpectBlocked maps resource key → a substring of the reason the plan
	// must refuse it. Every blocked change must be listed. A step with
	// blocked changes is not applied.
	ExpectBlocked map[string]string
	// ExpectNoop requires the plan to have no work at all.
	ExpectNoop bool
	// ExpectWrites is the exact, ordered list of writes the apply must make:
	// "create agent", "update skill", "destroy environment". nil skips the
	// check; an empty slice requires that nothing is written.
	ExpectWrites []string
	// ExpectKeys is the exact set of lockfile keys after the step. nil skips
	// the check; an empty slice requires an empty lockfile.
	ExpectKeys []string

	// Check runs after the apply (or after the plan, for a blocked step) for
	// anything the declarative fields cannot say.
	Check func(t *testing.T, fx *Fixture)
}

// Fixture is what a Check or BeforePlan callback can see and touch.
type Fixture struct {
	// Root is the directory the declared files live under.
	Root string
	// UniqueName is the per-test token substituted for {{name}}; see the
	// UniqueName function.
	UniqueName string
	// Lock is the lockfile as re-read from disk after the last apply, so a
	// Check sees what a later run would load.
	Lock *core.Lockfile
	// Plan is the current step's plan, not the re-plan that checks
	// convergence. BeforePlan sees the previous step's plan, or nil on the
	// first step.
	Plan *core.Plan
	// Unrecorded is the same client without the recorder, for out-of-band
	// edits that must not show up in ExpectWrites.
	Unrecorded core.Client

	registry  *core.Registry
	recording *recordingClient
}

// Entry returns the lockfile entry for a resource key, failing if untracked.
func (f *Fixture) Entry(t *testing.T, key string) *core.LockEntry {
	t.Helper()
	entry, ok := f.Lock.Get(key)
	require.True(t, ok, "%s is not in the lockfile", key)
	return entry
}

// ID returns the tracked ID for a resource key, failing if untracked.
func (f *Fixture) ID(t *testing.T, key string) string { return f.Entry(t, key).ID }

// Remote fetches the server's current copy of a tracked resource.
func (f *Fixture) Remote(t *testing.T, key string) map[string]any {
	t.Helper()
	entry := f.Entry(t, key)
	obj, err := f.Unrecorded.Get(context.Background(), entry.Kind, entry.ID)
	require.NoError(t, err)
	return obj
}

// Change returns the planned change for a key, failing if there is none.
func (f *Fixture) Change(t *testing.T, key string) *core.Change {
	t.Helper()
	c, ok := f.Plan.Change(key)
	require.True(t, ok, "no change planned for %s", key)
	return c
}

// Run drives steps against client in a fresh directory with its own lockfile.
// Each step is a subtest, and Run stops at the first that fails. The package
// doc describes the convergence check after each step and the teardown after
// the test.
func Run(t *testing.T, registry *core.Registry, client core.Client, steps []Step) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	lock, err := core.LoadLockfile(registry, filepath.Join(root, registry.LockfileName()))
	require.NoError(t, err)

	fx := &Fixture{
		Root:       root,
		UniqueName: UniqueName(t),
		Lock:       lock,
		Unrecorded: client,
		registry:   registry,
		recording:  &recordingClient{Client: client},
	}
	t.Cleanup(func() { teardown(t, fx) })

	for i, step := range steps {
		name := step.Name
		if name == "" {
			name = fmt.Sprintf("step %d", i+1)
		}
		if !t.Run(name, func(t *testing.T) { runStep(ctx, t, fx, step) }) {
			// Later steps assume earlier ones held; carrying on just buries
			// the first failure under its consequences.
			break
		}
	}
}

func runStep(ctx context.Context, t *testing.T, fx *Fixture, step Step) {
	for _, rel := range step.Remove {
		require.NoError(t, os.RemoveAll(filepath.Join(fx.Root, filepath.FromSlash(rel))))
	}
	WriteTree(t, fx.Root, Substitute(step.Write, fx.UniqueName))
	if step.BeforePlan != nil {
		step.BeforePlan(t, fx)
	}

	fx.recording.ResetWrites()
	fx.Plan = planTree(ctx, t, fx, step)

	for key, want := range step.ExpectActions {
		if c, ok := fx.Plan.Change(key); assert.True(t, ok, "no change planned for %s", key) {
			assert.Equal(t, want, c.Action, "action for %s", key)
		}
	}
	blocked := fx.Plan.Blocked()
	for _, c := range blocked {
		if want, ok := step.ExpectBlocked[c.Key]; assert.True(t, ok, "unexpectedly blocked: %s: %v", c.Key, c.Blocked) {
			assert.ErrorContains(t, c.Blocked, want)
		}
	}
	assert.Len(t, blocked, len(step.ExpectBlocked), "blocked changes")
	if step.ExpectNoop {
		assert.False(t, fx.Plan.HasWork(), "expected no work; plan was:\n%s", describe(fx.Plan))
	}

	// Apply refuses a plan with any blocked change, so there is nothing
	// further to run.
	if len(blocked) > 0 {
		if step.Check != nil {
			step.Check(t, fx)
		}
		return
	}

	hadWork := fx.Plan.HasWork()
	_, err := (&core.Applier{Client: fx.recording, Lock: fx.Lock}).Apply(ctx, fx.Plan)
	require.NoError(t, err)
	reloadLock(t, fx)

	if step.ExpectWrites != nil {
		assert.Equal(t, step.ExpectWrites, fx.recording.Writes())
	}
	if step.ExpectKeys != nil {
		assert.ElementsMatch(t, step.ExpectKeys, fx.Lock.Keys())
	}
	if step.Check != nil {
		step.Check(t, fx)
	}
	if t.Failed() || !hadWork {
		// A plan with no work re-plans identically; nothing to converge.
		return
	}

	// Re-plan and re-apply. Both must do nothing. The step's own options are
	// reused, so a pruned or forced apply is held to the same standard.
	fx.recording.ResetWrites()
	again := planTree(ctx, t, fx, step)
	if !assert.False(t, again.HasWork(), "apply did not converge; next plan was:\n%s", describe(again)) {
		return
	}
	assert.Empty(t, again.Blocked(), "apply converged but next plan is blocked")
	_, err = (&core.Applier{Client: fx.recording, Lock: fx.Lock}).Apply(ctx, again)
	require.NoError(t, err)
	assert.Empty(t, fx.recording.Writes(), "a converged apply must make no writes")
}

func planTree(ctx context.Context, t *testing.T, fx *Fixture, step Step) *core.Plan {
	t.Helper()
	loader := core.NewLoader(fx.registry, fx.Root, nil)
	require.NoError(t, loader.Add(ctx, []string{fx.Root}))
	require.NoError(t, loader.AddKeys(ctx, fx.Lock.Keys()))
	planner := &core.Planner{
		Registry: fx.registry,
		Client:   fx.recording,
		Lock:     fx.Lock,
		Prune:    step.Prune,
		Force:    step.Force,
	}
	plan, err := planner.Plan(ctx, loader)
	require.NoError(t, err)
	return plan
}

// reloadLock re-reads the lockfile from disk, proving it round-trips rather
// than only living in memory.
func reloadLock(t *testing.T, fx *Fixture) {
	t.Helper()
	lock, err := core.LoadLockfile(fx.registry, fx.Lock.Path)
	require.NoError(t, err)
	fx.Lock = lock
}

// teardown prunes everything the test tracked, then confirms each resource
// the test ever created is gone or archived — including ones a failed step
// orphaned from the lockfile.
func teardown(t *testing.T, fx *Fixture) {
	t.Helper()
	ctx := context.Background()
	if len(fx.Lock.Resources) > 0 {
		// An empty loader makes every tracked key an orphan, which is
		// exactly how `ant apply --prune` tears down a deleted tree.
		planner := &core.Planner{Registry: fx.registry, Client: fx.Unrecorded, Lock: fx.Lock, Prune: true}
		plan, err := planner.Plan(ctx, core.NewLoader(fx.registry, fx.Root, nil))
		if assert.NoError(t, err, "teardown plan") {
			_, err = (&core.Applier{Client: fx.Unrecorded, Lock: fx.Lock}).Apply(ctx, plan)
			assert.NoError(t, err, "teardown apply")
		}
	}
	for _, res := range fx.recording.created {
		obj, err := fx.Unrecorded.Get(ctx, res.kind, res.id)
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if !assert.NoError(t, err, "checking %s after teardown", res.id) {
			continue
		}
		if spec, _ := fx.registry.Spec(res.kind); spec.IsArchived(obj) {
			continue
		}
		t.Errorf("%s %s survived teardown; destroying it directly", res.kind, res.id)
		if err := fx.Unrecorded.Destroy(ctx, res.kind, res.id); err != nil {
			t.Logf("%s %s is left on the server: %v", res.kind, res.id, err)
		}
	}
}

// WriteTree materializes slash-pathed files under root.
func WriteTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}
}

// UniqueName derives a token from the test name and ANT_ACC_RUN_ID for values
// the server requires to be unique, like a skill's display name. It is stable
// for a given test and run ID so a failure reproduces with the same names; set
// a fresh ANT_ACC_RUN_ID when an earlier run left debris behind.
func UniqueName(t *testing.T) string {
	sum := sha256.Sum256([]byte(t.Name() + "\x00" + os.Getenv("ANT_ACC_RUN_ID")))
	return "anttest-" + hex.EncodeToString(sum[:])[:10]
}

// Substitute returns a copy of files with each "{{name}}" in the content
// replaced by name.
func Substitute(files map[string]string, name string) map[string]string {
	out := make(map[string]string, len(files))
	for rel, content := range files {
		out[rel] = strings.ReplaceAll(content, "{{name}}", name)
	}
	return out
}

func describe(plan *core.Plan) string {
	var b strings.Builder
	(&render.Renderer{Out: &b, Verbose: true}).RenderPlan(plan)
	return b.String()
}
