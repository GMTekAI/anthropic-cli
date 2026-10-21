package cmd

import (
	"testing"

	"github.com/anthropics/anthropic-cli/v2/internal/mocktest"
)

func TestBetaOrganizationFederationRulesWorkspacesList(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"beta:organization:federation:rules:workspaces", "list",
			"--max-items", "10",
			"--federation-rule-id", "federation_rule_id",
			"--limit", "1",
			"--page", "page",
			"--beta", "message-batches-2024-09-24",
		)
	})
}

func TestBetaOrganizationFederationRulesWorkspacesAdd(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"beta:organization:federation:rules:workspaces", "add",
			"--federation-rule-id", "federation_rule_id",
			"--workspace-id", "workspace_id",
			"--beta", "message-batches-2024-09-24",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("workspace_id: workspace_id")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"beta:organization:federation:rules:workspaces", "add",
			"--federation-rule-id", "federation_rule_id",
			"--beta", "message-batches-2024-09-24",
		)
	})
}

func TestBetaOrganizationFederationRulesWorkspacesRemove(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"beta:organization:federation:rules:workspaces", "remove",
			"--federation-rule-id", "federation_rule_id",
			"--workspace-id", "workspace_id",
			"--beta", "message-batches-2024-09-24",
		)
	})
}
