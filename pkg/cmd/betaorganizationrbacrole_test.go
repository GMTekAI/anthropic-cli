package cmd

import (
	"testing"

	"github.com/anthropics/anthropic-cli/v2/internal/mocktest"
)

func TestBetaOrganizationRBACRolesRetrieve(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--base-url", mocktest.ServerWithQuery(t, "beta=true"),
			"beta:organization:rbac-roles", "retrieve",
			"--rbac-role-id", "rbac_role_id",
		)
	})
}

func TestBetaOrganizationRBACRolesList(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--base-url", mocktest.ServerWithQuery(t, "beta=true"),
			"beta:organization:rbac-roles", "list",
			"--max-items", "10",
			"--limit", "1",
			"--page", "eyJjdXJzb3IiOiAicmJhY19yb2xlXzAxIn0",
		)
	})
}
