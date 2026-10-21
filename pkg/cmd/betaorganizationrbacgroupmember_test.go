package cmd

import (
	"testing"

	"github.com/anthropics/anthropic-cli/v2/internal/mocktest"
)

func TestBetaOrganizationRBACGroupsMembersList(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"beta:organization:rbac-groups:members", "list",
			"--max-items", "10",
			"--rbac-group-id", "rbac_group_id",
			"--limit", "1",
			"--page", "eyJjdXJzb3IiOiAicmJhY19ncm91cF8wMSJ9",
		)
	})
}

func TestBetaOrganizationRBACGroupsMembersAdd(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"beta:organization:rbac-groups:members", "add",
			"--rbac-group-id", "rbac_group_id",
			"--user-id", "user_01WCz1FkmYMm4gnmykNKUu3Q",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("user_id: user_01WCz1FkmYMm4gnmykNKUu3Q")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"beta:organization:rbac-groups:members", "add",
			"--rbac-group-id", "rbac_group_id",
		)
	})
}

func TestBetaOrganizationRBACGroupsMembersRemove(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"beta:organization:rbac-groups:members", "remove",
			"--rbac-group-id", "rbac_group_id",
			"--user-id", "user_id",
		)
	})
}
