package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/anthropics/anthropic-cli/internal/declarative/acctest"
	"github.com/anthropics/anthropic-cli/internal/declarative/claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanPromptRefusesWithoutAHuman(t *testing.T) {
	// A script that forgot --yes is asking for a plan, not for its resources
	// to be rewritten unattended.
	require.NoError(t, canPrompt(true, false))

	for _, tc := range []struct {
		name             string
		stdoutIsTerminal bool
		stdinIsPiped     bool
	}{
		{"output is redirected", false, false},
		{"input is piped", true, true},
		{"neither", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := canPrompt(tc.stdoutIsTerminal, tc.stdinIsPiped)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--yes")
			assert.Contains(t, err.Error(), "--dry-run")
		})
	}
}

// TestAccApplyCommand drives the real binary against the real API through a
// cold start, a converged re-plan and a prune. The reconciler has its own
// acceptance suite; this covers what only the command owns — flag wiring,
// credential resolution from the active profile, lockfile placement, the
// --yes gate, exit codes, and the rendered plan.
//
// It runs a subprocess because the package-level Command tree keeps parsed
// flag state between in-process runs.
func TestAccApplyCommand(t *testing.T) {
	acctest.SkipUnlessEnabled(t)
	bin := buildAnt(t)
	tree := t.TempDir()
	lockPath := filepath.Join(tree, claude.Registry().LockfileName())
	acctest.WriteTree(t, tree, acctest.Substitute(map[string]string{
		"skills/greeter/SKILL.md": "---\nname: greeter\ndescription: says hello\ndisplay_title: {{name}}\n---\nSay hello.\n",
		"agents/concierge.md":     "---\nname: {{name}}\nmodel: claude-sonnet-4-5\nskills:\n  - ../skills/greeter\n---\nBe helpful.\n",
		"environments/lobby.yml":  "name: {{name}}\n",
	}, acctest.UniqueName(t)))

	ant := func(args ...string) (string, error) {
		cmd := exec.Command(bin, append([]string{"apply", "--lock-file", lockPath}, args...)...)
		cmd.Dir = tree
		// apply creates its remote-skill cache on every run; keep that out of
		// the developer's own cache directory.
		cmd.Env = append(os.Environ(), "XDG_CACHE_HOME="+t.TempDir(), "NO_COLOR=1")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	t.Cleanup(func() {
		// With the files gone every tracked key is an orphan, so --prune tears
		// down whatever the test created, whichever step it stopped at.
		for _, dir := range []string{"skills", "agents", "environments"} {
			_ = os.RemoveAll(filepath.Join(tree, dir))
		}
		if out, err := ant("--yes", "--prune"); err != nil {
			t.Logf("cleanup: %v\n%s", err, out)
		}
	})

	out, err := ant("--dry-run", tree)
	require.NoError(t, err, out)
	assert.Contains(t, out, "No lockfile yet")
	assert.Contains(t, out, "+ ./agents/concierge.md")
	assert.Contains(t, out, "Plan: 3 to create, 0 to update, 0 to remove, 0 unchanged.")
	assert.NoFileExists(t, lockPath, "--dry-run must not write state")

	out, err = ant(tree)
	require.Error(t, err, "without a terminal and without --yes, apply must refuse")
	assert.Contains(t, out, "--yes")
	assert.NoFileExists(t, lockPath)

	out, err = ant("--yes", tree)
	require.NoError(t, err, out)
	assert.Contains(t, out, "Apply complete: 3 created, 0 updated, 0 removed, 0 unchanged.")
	assert.FileExists(t, lockPath)

	// No paths: everything tracked is reconciled, and it has converged.
	out, err = ant()
	require.NoError(t, err, out)
	assert.Contains(t, out, "Found lockfile at")
	assert.Contains(t, out, "Everything is up to date.")

	require.NoError(t, os.RemoveAll(filepath.Join(tree, "agents")))
	out, err = ant("--yes")
	require.NoError(t, err, out)
	assert.Contains(t, out, "--prune", "an orphan is reported with the flag that removes it")
	assert.Contains(t, out, "Everything is up to date.")

	out, err = ant("--yes", "--prune")
	require.NoError(t, err, out)
	assert.Contains(t, out, "- ./agents/concierge.md")
	assert.Contains(t, out, "0 created, 0 updated, 1 removed")
	data, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "concierge")
	assert.Contains(t, string(data), "greeter")
}

// buildAnt compiles the CLI once for the whole test. go run would relink on
// every invocation and add its own exit-status line to the output under test.
// Tests run in their package directory, so the module root is two levels up.
func buildAnt(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "ant")
	build := exec.Command("go", "build", "-o", bin, "./cmd/ant")
	build.Dir = "../.."
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build: %s", out)
	return bin
}
