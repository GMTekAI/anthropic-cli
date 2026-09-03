package claude

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropics/anthropic-cli/internal/declarative/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFirstApplyCreatesEverythingInDependencyOrder(t *testing.T) {
	h := newHarness(t, basicTree())

	res := h.apply()
	assert.Equal(t, 4, res.Created)
	assert.Equal(t, []string{"create skill", "create environment", "create agent", "create agent"}, h.client.calls)

	// Every resource is now tracked, with the IDs the fake handed out.
	assert.ElementsMatch(t, []string{
		"./agents/code-reviewer.md", "./agents/code-verifier.md",
		"./environments/cloud.yml", "./skills/pr-writer",
	}, h.lock.Keys())

	reviewer, ok := h.lock.Get("./agents/code-reviewer.md")
	require.True(t, ok)
	assert.Equal(t, "1", reviewer.Version)
	assert.NotEmpty(t, reviewer.Hash)
	assert.NotEmpty(t, reviewer.RemoteHash)
}

func TestSecondApplyIsANoop(t *testing.T) {
	h := newHarness(t, basicTree())
	h.apply()
	h.client.calls = nil

	plan := h.plan()
	assert.False(t, plan.HasWork(), "a repeated apply must converge")
	_, _, _, noop := plan.Counts()
	assert.Equal(t, 4, noop)

	h.apply()
	assert.Empty(t, h.client.calls, "a converged apply must not touch the API")
}

func TestReferencesArePinnedToConcreteVersions(t *testing.T) {
	h := newHarness(t, basicTree())
	h.apply()

	reviewerID := h.lock.Resources["./agents/code-reviewer.md"].ID
	obj := h.client.objects[reviewerID]

	skills := obj["skills"].([]any)
	require.Len(t, skills, 1)
	skill := skills[0].(map[string]any)
	assert.Equal(t, "custom", skill["type"])
	assert.Equal(t, h.lock.Resources["./skills/pr-writer"].ID, skill["skill_id"])
	assert.Equal(t, h.lock.Resources["./skills/pr-writer"].Version, skill["version"])

	roster := obj["multiagent"].(map[string]any)["agents"].([]any)
	require.Len(t, roster, 1)
	entry := roster[0].(map[string]any)
	assert.Equal(t, h.lock.Resources["./agents/code-verifier.md"].ID, entry["id"])
	assert.Equal(t, int64(1), entry["version"])
}

// A pinned reference has to move when its target does. Otherwise editing a
// sub-agent leaves its coordinator quietly running the previous prompt, with
// nothing in the plan to say so.
func TestEditingADependencyRepinsItsDependents(t *testing.T) {
	h := newHarness(t, basicTree())
	h.apply()
	h.client.calls = nil

	require.NoError(t, os.WriteFile(
		filepath.Join(h.root, "agents/code-verifier.md"),
		[]byte("---\nmodel: claude-sonnet-5\n---\nverify differently\n"), 0o644))

	plan := h.plan()
	assert.Equal(t, core.ActionUpdate, actionFor(plan, "./agents/code-verifier.md"))
	assert.Equal(t, core.ActionUpdate, actionFor(plan, "./agents/code-reviewer.md"),
		"the coordinator must follow its roster")
	assert.Equal(t, core.ActionNoop, actionFor(plan, "./skills/pr-writer"))

	h.apply()

	reviewerID := h.lock.Resources["./agents/code-reviewer.md"].ID
	roster := h.client.objects[reviewerID]["multiagent"].(map[string]any)["agents"].([]any)
	assert.Equal(t, int64(2), roster[0].(map[string]any)["version"],
		"the roster must point at the new version, not the one it was created with")

	// And it converges: no endless re-pinning.
	plan = h.plan()
	assert.False(t, plan.HasWork())
}

func TestUnpinnedReferenceDoesNotCascade(t *testing.T) {
	files := basicTree()
	files["agents/code-reviewer.md"] = `---
model: claude-sonnet-5
skills:
  - path: ../skills/pr-writer
    version: latest
---
review
`
	h := newHarness(t, files)
	h.apply()

	require.NoError(t, os.WriteFile(
		filepath.Join(h.root, "skills/pr-writer/SKILL.md"),
		[]byte("---\nname: pr-writer\ndescription: d\n---\nnew guidance\n"), 0o644))

	plan := h.plan()
	assert.Equal(t, core.ActionUpdate, actionFor(plan, "./skills/pr-writer"))
	assert.Equal(t, core.ActionNoop, actionFor(plan, "./agents/code-reviewer.md"),
		"an explicitly unpinned reference tracks latest and needs no rewrite")
}

func TestOutOfBandChangeBlocksTheApply(t *testing.T) {
	h := newHarness(t, basicTree())
	h.apply()

	// Somebody edits the agent in the console.
	verifierID := h.lock.Resources["./agents/code-verifier.md"].ID
	h.client.objects[verifierID]["description"] = "edited elsewhere"
	h.client.objects[verifierID]["version"] = int64(2)

	plan := h.plan()
	blocked := plan.Blocked()
	require.Len(t, blocked, 1)
	assert.Equal(t, "./agents/code-verifier.md", blocked[0].Key)
	assert.ErrorContains(t, blocked[0].Blocked, "--force")

	applier := &core.Applier{Client: h.client, Lock: h.lock}
	_, err := applier.Apply(context.Background(), plan)
	require.ErrorContains(t, err, "changed outside this config")

	// With the flag, it proceeds and reasserts the desired state.
	h.apply(withForce)
	assert.NotEqual(t, "edited elsewhere", h.client.objects[verifierID]["description"])
}

// A vanished resource can only be replaced, and replacing it mints a new ID
// that breaks anything referencing the old one from outside this config. That
// is at least as consequential as overwriting fields, so it is gated by the
// same flag rather than done silently.
func TestDeletedRemoteResourceNeedsForceToBeRecreated(t *testing.T) {
	h := newHarness(t, basicTree())
	h.apply()
	oldID := h.lock.Resources["./environments/cloud.yml"].ID
	delete(h.client.objects, oldID)

	plan := h.plan()
	assert.Equal(t, core.ActionCreate, actionFor(plan, "./environments/cloud.yml"))
	blocked := plan.Blocked()
	require.Len(t, blocked, 1)
	assert.ErrorContains(t, blocked[0].Blocked, "--force")
	assert.ErrorContains(t, blocked[0].Blocked, oldID)
	assert.Equal(t, oldID, blocked[0].Replaces, "a recreate says what it replaces")

	h.apply(withForce)
	assert.NotEqual(t, oldID, h.lock.Resources["./environments/cloud.yml"].ID)
}

// A file rewritten into something the server already holds — an empty list
// added, a value respelled the way the server normalizes it — has a new
// fingerprint but nothing to send. That is not an update.
func TestRespellingAFileIntoWhatTheServerHasIsNotAChange(t *testing.T) {
	h := newHarness(t, basicTree())
	h.apply()
	entry := h.lock.Resources["./environments/cloud.yml"]
	before := entry.Hash
	// Stand in for a server that stored the value normalized at create time
	// (as allowed_hosts are lower-cased): the remote reads "SANDBOX" and
	// always did, so there is no drift to speak of.
	h.client.objects[entry.ID]["description"] = "SANDBOX"
	entry.RemoteHash = ""
	require.NoError(t, h.lock.Save())

	require.NoError(t, os.WriteFile(filepath.Join(h.root, "environments/cloud.yml"),
		[]byte("description: SANDBOX\n"), 0o644))
	plan := h.plan()
	assert.Equal(t, core.ActionNoop, actionFor(plan, "./environments/cloud.yml"))
	assert.False(t, plan.HasWork())

	// Applying records the new fingerprint, so the diff is not recomputed on
	// every later plan.
	h.apply()
	assert.NotEqual(t, before, h.lock.Resources["./environments/cloud.yml"].Hash)
	assert.Empty(t, h.client.calls[4:], "nothing was sent")
}

// Metadata is declared, not merged: keys the file drops — or that someone
// added elsewhere — are removals, and the plan must show them as such rather
// than as an update with nothing under it.
func TestDroppedMetadataKeysShowAsRemovals(t *testing.T) {
	tree := basicTree()
	tree["environments/cloud.yml"] = "description: sandbox\nmetadata: {team: alpha, owner: file}\n"
	h := newHarness(t, tree)
	h.apply()
	id := h.lock.Resources["./environments/cloud.yml"].ID
	h.client.objects[id]["metadata"].(map[string]any)["ticket"] = "OPS-1"

	require.NoError(t, os.WriteFile(filepath.Join(h.root, "environments/cloud.yml"),
		[]byte("description: sandbox\nmetadata: {team: alpha}\n"), 0o644))
	plan := h.plan(withForce)
	var change *core.Change
	for _, c := range plan.Changes {
		if c.Key == "./environments/cloud.yml" {
			change = c
		}
	}
	require.Equal(t, core.ActionUpdate, change.Action)
	require.NotNil(t, change.Diff)
	bag := change.Diff.Fields["metadata"]
	require.NotNil(t, bag)
	assert.Equal(t, core.DiffRemoved, bag.Fields["owner"].Kind)
	assert.Equal(t, core.DiffRemoved, bag.Fields["ticket"].Kind)
	assert.NotContains(t, bag.Fields, "team")

	h.apply(withForce)
	assert.Equal(t, map[string]any{"team": "alpha"}, h.client.objects[id]["metadata"])
}

func TestArchivedRemoteResourceNeedsForceToBeRecreated(t *testing.T) {
	h := newHarness(t, basicTree())
	h.apply()
	id := h.lock.Resources["./environments/cloud.yml"].ID
	h.client.objects[id]["archived_at"] = "2026-01-01T00:00:00Z"

	plan := h.plan()
	var change *core.Change
	for _, c := range plan.Changes {
		if c.Key == "./environments/cloud.yml" {
			change = c
		}
	}
	require.NotNil(t, change)
	assert.Equal(t, core.ActionCreate, change.Action)
	assert.Contains(t, change.Reasons[0], "archived")
	require.Error(t, change.Blocked)
	assert.ErrorContains(t, change.Blocked, "--force")

	h.apply(withForce)
	assert.NotEqual(t, id, h.lock.Resources["./environments/cloud.yml"].ID)
}

func TestRemovedFilesAreOnlyDestroyedWithPrune(t *testing.T) {
	h := newHarness(t, basicTree())
	h.apply()
	envID := h.lock.Resources["./environments/cloud.yml"].ID

	require.NoError(t, os.Remove(filepath.Join(h.root, "environments/cloud.yml")))

	plan := h.plan()
	assert.False(t, plan.HasWork(), "an orphan is reported, not silently destroyed")
	require.Len(t, plan.Warnings, 1)
	assert.Contains(t, plan.Warnings[0], "--prune")

	h.client.calls = nil
	h.apply(withPrune)
	assert.Equal(t, []string{"destroy environment " + envID}, h.client.calls)
	assert.NotContains(t, h.lock.Keys(), "./environments/cloud.yml")
}

func TestPruneDestroysDependentsFirst(t *testing.T) {
	h := newHarness(t, basicTree())
	h.apply()
	require.NoError(t, os.RemoveAll(filepath.Join(h.root, "agents")))
	require.NoError(t, os.RemoveAll(filepath.Join(h.root, "skills")))

	h.client.calls = nil
	h.apply(withPrune)

	require.Len(t, h.client.calls, 3)
	assert.Contains(t, h.client.calls[0], "destroy agent")
	assert.Contains(t, h.client.calls[1], "destroy agent")
	assert.Contains(t, h.client.calls[2], "destroy skill",
		"a skill must outlive the agents that reference it")
}

func TestSkillChangeUploadsANewVersion(t *testing.T) {
	h := newHarness(t, basicTree())
	h.apply()
	before := h.lock.Resources["./skills/pr-writer"].Version

	require.NoError(t, os.WriteFile(
		filepath.Join(h.root, "skills/pr-writer/reference.md"), []byte("more detail\n"), 0o644))

	h.client.calls = nil
	h.apply()
	assert.Contains(t, h.client.calls, "create skill version")
	assert.NotEqual(t, before, h.lock.Resources["./skills/pr-writer"].Version)
}

func TestPartialFailureStillRecordsWhatSucceeded(t *testing.T) {
	h := newHarness(t, basicTree())
	h.client.failOn = "create agent"

	loader := core.NewLoader(Registry(), h.root, nil)
	require.NoError(t, loader.Add(context.Background(), []string{h.root}))
	planner := &core.Planner{Registry: Registry(), Client: h.client, Lock: h.lock}
	plan, err := planner.Plan(context.Background(), loader)
	require.NoError(t, err)

	applier := &core.Applier{Client: h.client, Lock: h.lock}
	_, err = applier.Apply(context.Background(), plan)
	require.Error(t, err)

	// The skill and environment were created before the failure, and their IDs
	// must survive — otherwise the next apply creates duplicates.
	h.reloadLock()
	assert.ElementsMatch(t, []string{"./environments/cloud.yml", "./skills/pr-writer"}, h.lock.Keys())

	h.client.failOn = ""
	h.client.calls = nil
	h.apply()
	assert.Equal(t, []string{"create agent", "create agent"}, h.client.calls,
		"the retry must only create what is missing")
}

func TestClearedFieldIsExplicitlyNulled(t *testing.T) {
	files := basicTree()
	files["environments/cloud.yml"] = "description: sandbox\n"
	h := newHarness(t, files)
	h.apply()

	require.NoError(t, os.WriteFile(filepath.Join(h.root, "environments/cloud.yml"),
		[]byte("name: cloud\n"), 0o644))

	plan := h.plan()
	var change *core.Change
	for _, c := range plan.Changes {
		if c.Key == "./environments/cloud.yml" {
			change = c
		}
	}
	require.NotNil(t, change)
	require.Equal(t, core.ActionUpdate, change.Action)
	require.NotNil(t, change.Diff)
	var removed []string
	for name, d := range change.Diff.Fields {
		if d.Kind == core.DiffRemoved {
			removed = append(removed, name)
		}
	}
	assert.Equal(t, []string{"description"}, removed)

	h.apply()
	id := h.lock.Resources["./environments/cloud.yml"].ID
	assert.NotContains(t, h.client.objects[id], "description")
}

// Every tracked resource missing at once is not deletion; it is credentials
// for somewhere else. Planning must stop rather than offer to recreate the lot.
func TestAllResourcesMissingIsRefusedAsAWrongTarget(t *testing.T) {
	h := newHarness(t, basicTree())
	h.apply()
	h.client.objects = map[string]map[string]any{} // a different, empty organization

	loader := core.NewLoader(Registry(), h.root, nil)
	require.NoError(t, loader.Add(context.Background(), []string{h.root}))
	planner := &core.Planner{Registry: Registry(), Client: h.client, Lock: h.lock}
	_, err := planner.Plan(context.Background(), loader)
	require.ErrorContains(t, err, "none of the 4 resources")

	// --force is how the user says they really were all deleted.
	planner.Force = true
	plan, err := planner.Plan(context.Background(), loader)
	require.NoError(t, err)
	create, _, _, _ := plan.Counts()
	assert.Equal(t, 4, create)
}
