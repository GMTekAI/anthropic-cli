package cmd

import (
	"testing"

	"github.com/anthropics/anthropic-cli/v2/internal/mocktest"
)

func TestOrganizationServiceAccountsWorkspacesList(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"organization:service-accounts:workspaces", "list",
			"--max-items", "10",
			"--service-account-id", "service_account_id",
			"--limit", "1",
			"--page", "page",
		)
	})
}

func TestOrganizationServiceAccountsWorkspacesAdd(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"organization:service-accounts:workspaces", "add",
			"--service-account-id", "service_account_id",
			"--workspace-id", "workspace_id",
			"--workspace-role", "workspace_admin",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"workspace_id: workspace_id\n" +
			"workspace_role: workspace_admin\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"organization:service-accounts:workspaces", "add",
			"--service-account-id", "service_account_id",
		)
	})
}

func TestOrganizationServiceAccountsWorkspacesRemove(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"organization:service-accounts:workspaces", "remove",
			"--service-account-id", "service_account_id",
			"--workspace-id", "workspace_id",
		)
	})
}
