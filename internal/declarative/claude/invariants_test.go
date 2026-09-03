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

// Rules that are easy to break by accident, each stated as the invariant it
// protects rather than as the mistake that motivated it.

func TestReadmeInAKindDirectoryIsNotAResource(t *testing.T) {
	// A kind-named directory is a hint, not a declaration. Treating it as one
	// turns every README and design note in agents/ into a real agent whose
	// system prompt is that prose.
	root := writeTree(t, map[string]string{
		"agents/README.md":       "# How to add an agent\n\nDrop a file in here.\n",
		"agents/real.md":         "---\nmodel: m\n---\nprompt\n",
		"environments/notes.md":  "Some prose about environments.\n",
		"deployments/schema.yml": "- not\n- a\n- mapping\n",
	})
	l := core.NewLoader(Registry(), root, nil)
	require.NoError(t, l.Add(context.Background(), []string{root}))

	keys := make([]string, 0, len(l.Sources()))
	for k := range l.Sources() {
		keys = append(keys, k)
	}
	assert.ElementsMatch(t, []string{"./agents/real.md"}, keys)
}

func TestSkillTitleChangeIsRefusedRatherThanFaked(t *testing.T) {
	// Skills have no update endpoint, and a version upload carries only files.
	// A title change therefore cannot be applied at all — reporting success
	// for it is worse than refusing.
	h := newHarness(t, map[string]string{
		"skills/pr-writer/SKILL.md": "---\nname: pr-writer\ndisplay_name: PDF Tools\n---\nbody\n",
	})
	h.apply()

	require.NoError(t, os.WriteFile(filepath.Join(h.root, "skills/pr-writer/SKILL.md"),
		[]byte("---\nname: pr-writer\ndisplay_name: PDF Utilities\n---\nbody\n"), 0o644))

	plan := h.plan()
	blocked := plan.Blocked()
	require.Len(t, blocked, 1)
	assert.ErrorContains(t, blocked[0].Blocked, "display_name cannot be changed after the resource is created")
}

func TestSkillRenameIsRefusedRatherThanFailedMidApply(t *testing.T) {
	// The API holds every version of a skill to the `name` its first SKILL.md
	// declared and rejects an upload that changes it. Letting the plan call
	// that an update would fail halfway through the run; it is a replace, and
	// replacing is the user's decision.
	skillMD := func(name string) string {
		return "---\nname: " + name + "\ndisplay_name: PR Writer\ndescription: d\n---\nhow to write\n"
	}
	tree := basicTree()
	tree["skills/pr-writer/SKILL.md"] = skillMD("pr-writer")
	h := newHarness(t, tree)
	h.apply()

	require.NoError(t, os.WriteFile(filepath.Join(h.root, "skills/pr-writer/SKILL.md"), []byte(skillMD("pr-author")), 0o644))

	plan := h.plan()
	blocked := plan.Blocked()
	require.Len(t, blocked, 1)
	assert.Equal(t, "./skills/pr-writer", blocked[0].Key)
	assert.EqualError(t, blocked[0].Blocked, "name cannot be changed after the resource is created; delete ./skills/pr-writer and apply again to replace it")

	// A server that does not report the name leaves nothing to compare, so the
	// judgement passes to the API rather than blocking every edit.
	id := h.lock.Resources["./skills/pr-writer"].ID
	delete(h.client.objects[id], "name")
	plan = h.plan()
	assert.Empty(t, plan.Blocked())
	assert.Equal(t, core.ActionUpdate, actionFor(plan, "./skills/pr-writer"))
}

func TestWriteOnlyValuesInsideArraysAreNeverPrinted(t *testing.T) {
	// Diffs stop at the array, so the reported path is "resources" — the
	// "resources[].authorization_token" marker has to shield the whole field,
	// or a formatter prints the token in cleartext.
	change := &core.Change{
		Kind:      KindDeployment,
		Sensitive: map[string]bool{"resources[].authorization_token": true},
	}
	assert.True(t, change.IsSensitive("resources"))
	assert.True(t, change.IsSensitive("resources.authorization_token"))
	assert.False(t, change.IsSensitive("name"))
}

func TestAgentUpdateWithoutAVersionIsAnError(t *testing.T) {
	// Sending no version would either 400 unhelpfully or, worse, skip the
	// concurrent-write check the field exists for.
	h := newHarness(t, basicTree())
	h.apply()

	// Simulate a read that came back without a version.
	loader := core.NewLoader(Registry(), h.root, nil)
	require.NoError(t, loader.Add(context.Background(), []string{h.root}))
	planner := &core.Planner{Registry: Registry(), Client: h.client, Lock: h.lock}
	plan, err := planner.Plan(context.Background(), loader)
	require.NoError(t, err)

	var change *core.Change
	for _, c := range plan.Changes {
		if c.Key == "./agents/code-verifier.md" {
			change = c
		}
	}
	require.NotNil(t, change)
	change.Action = core.ActionUpdate
	delete(change.Remote, "version")

	applier := &core.Applier{Client: h.client, Lock: h.lock}
	_, err = applier.Apply(context.Background(), plan)
	require.ErrorContains(t, err, "did not report a current version")
}

func TestSkillHashAndUploadComeFromOneWalk(t *testing.T) {
	// The recorded hash must describe the bytes that were actually uploaded.
	// If it is computed from a second, later walk, a file that appears between
	// plan and apply lands in the hash but not the bundle — and is then never
	// uploaded, because every future plan sees a matching hash.
	h := newHarness(t, basicTree())
	h.apply()

	loader := core.NewLoader(Registry(), h.root, nil)
	require.NoError(t, loader.Add(context.Background(), []string{h.root}))
	require.NoError(t, loader.AddKeys(context.Background(), h.lock.Keys()))
	planner := &core.Planner{Registry: Registry(), Client: h.client, Lock: h.lock}
	plan, err := planner.Plan(context.Background(), loader)
	require.NoError(t, err)

	// A file appears while the plan is sitting at the confirmation prompt.
	require.NoError(t, os.WriteFile(
		filepath.Join(h.root, "skills/pr-writer/late.md"), []byte("late\n"), 0o644))

	applier := &core.Applier{Client: h.client, Lock: h.lock}
	_, err = applier.Apply(context.Background(), plan)
	require.NoError(t, err)
	h.reloadLock()

	// The late file was not uploaded, so the next plan must still want to.
	plan = h.plan()
	assert.Equal(t, core.ActionUpdate, actionFor(plan, "./skills/pr-writer"),
		"a file that missed the upload must not be recorded as applied")
}

func TestChangingAResourcesKindIsRefused(t *testing.T) {
	// Creating the new resource while forgetting the old one leaves it on the
	// server with nothing tracking it — no later apply, not even --prune, can
	// see it again.
	h := newHarness(t, map[string]string{
		"things/x.yml": "type: environment\ndescription: an environment\n",
	})
	h.apply()
	oldID := h.lock.Resources["./things/x.yml"].ID

	require.NoError(t, os.WriteFile(filepath.Join(h.root, "things/x.yml"),
		[]byte("type: agent\nmodel: claude-sonnet-5\n"), 0o644))

	plan := h.plan()
	blocked := plan.Blocked()
	require.Len(t, blocked, 1)
	assert.ErrorContains(t, blocked[0].Blocked, oldID)
	assert.ErrorContains(t, blocked[0].Blocked, "--prune")
}

func TestRemovedMetadataKeysArePruned(t *testing.T) {
	h := newHarness(t, map[string]string{
		"environments/e.yml": "description: d\nmetadata:\n  owner: platform\n  team: infra\n",
	})
	h.apply()
	id := h.lock.Resources["./environments/e.yml"].ID

	require.NoError(t, os.WriteFile(filepath.Join(h.root, "environments/e.yml"),
		[]byte("description: d\nmetadata:\n  owner: platform\n"), 0o644))
	h.apply()

	meta := h.client.objects[id]["metadata"].(map[string]any)
	assert.Equal(t, "platform", meta["owner"])
	assert.NotContains(t, meta, "team")
}
