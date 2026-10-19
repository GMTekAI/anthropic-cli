package cmd

import (
	"testing"

	"github.com/anthropics/anthropic-cli/internal/mocktest"
	"github.com/anthropics/anthropic-cli/internal/requestflag"
)

func TestBetaOrganizationWorkspacesRateLimitsUpdate(t *testing.T) {
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"beta:organization:workspaces:rate-limits", "update",
			"--workspace-id", "workspace_id",
			"--rate-limit-id", "rate_limit_id",
			"--limit", "{type: fast_itpmca, value: 0}",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(betaOrganizationWorkspacesRateLimitsUpdate)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"beta:organization:workspaces:rate-limits", "update",
			"--workspace-id", "workspace_id",
			"--rate-limit-id", "rate_limit_id",
			"--limit.type", "fast_itpmca",
			"--limit.value", "0",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"limits:\n" +
			"  - type: fast_itpmca\n" +
			"    value: 0\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"beta:organization:workspaces:rate-limits", "update",
			"--workspace-id", "workspace_id",
			"--rate-limit-id", "rate_limit_id",
		)
	})
}

func TestBetaOrganizationWorkspacesRateLimitsList(t *testing.T) {
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"beta:organization:workspaces:rate-limits", "list",
			"--max-items", "10",
			"--workspace-id", "workspace_id",
			"--group-type", "batch",
			"--include-inherited=true",
			"--limit", "1",
			"--page", "page",
		)
	})
}
