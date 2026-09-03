package claude

import (
	"testing"

	"github.com/anthropics/anthropic-cli/internal/declarative/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The per-field schema is flattened into the lists the reconciler consults.
// This pins the flattening, because a field silently landing in the wrong
// bucket is invisible until it costs a customer a broken plan: a missed
// Computed shows up as drift that never clears, and a missed WriteOnly prints
// a secret.
func TestAgentSchemaFlattensAsExpected(t *testing.T) {
	spec, ok := Registry().Spec(KindAgent)
	require.True(t, ok)

	assert.ElementsMatch(t,
		[]string{"id", "type", "created_at", "updated_at", "archived_at", "version"},
		spec.Computed())
	assert.ElementsMatch(t,
		[]string{"description", "system", "skills", "tools", "mcp_servers", "multiagent"},
		spec.Clearable())
	assert.Empty(t, spec.WriteOnly())

	paths := map[string]bool{}
	for _, slot := range spec.RefSlots() {
		paths[slot.Path] = true
	}
	assert.Equal(t, map[string]bool{"skills": true, "multiagent.agents": true}, paths)
}

func TestDeploymentTokenIsWriteOnly(t *testing.T) {
	// The one secret left in the schema. If this stops being write-only the
	// plan renders a GitHub token in cleartext.
	spec, ok := Registry().Spec(KindDeployment)
	require.True(t, ok)
	assert.Equal(t, []string{"resources[].authorization_token"}, spec.WriteOnly())
}

func TestKindOrderIsTopological(t *testing.T) {
	// Destroy ordering and plan tie-breaking rely on a kind appearing after
	// everything it can reference.
	r := Registry()
	position := map[core.Kind]int{}
	for i, k := range r.Kinds() {
		position[k] = i
	}
	for _, k := range r.Kinds() {
		spec, _ := r.Spec(k)
		for _, slot := range spec.RefSlots() {
			for _, allowed := range slot.To {
				if allowed == k {
					continue // self-reference orders against itself
				}
				assert.Less(t, position[allowed], position[k],
					"%s references %s, so %s must be registered first", k, allowed, allowed)
			}
		}
	}
}
