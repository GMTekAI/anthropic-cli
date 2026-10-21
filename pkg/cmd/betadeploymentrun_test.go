package cmd

import (
	"testing"

	"github.com/anthropics/anthropic-cli/v2/internal/mocktest"
)

func TestBetaDeploymentRunsRetrieve(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"beta:deployment-runs", "retrieve",
			"--deployment-run-id", "deployment_run_id",
			"--beta", "message-batches-2024-09-24",
			"--workspace-id", "wrkspc_011CZkZaBF1tNoB5wlCeusgy",
		)
	})
}

func TestBetaDeploymentRunsList(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"beta:deployment-runs", "list",
			"--max-items", "10",
			"--created-at-gt", "'2019-12-27T18:11:19.117Z'",
			"--created-at-gte", "'2019-12-27T18:11:19.117Z'",
			"--created-at-lt", "'2019-12-27T18:11:19.117Z'",
			"--created-at-lte", "'2019-12-27T18:11:19.117Z'",
			"--deployment-id", "deployment_id",
			"--has-error=true",
			"--limit", "0",
			"--page", "page",
			"--trigger-type", "schedule",
			"--beta", "message-batches-2024-09-24",
			"--workspace-id", "wrkspc_011CZkZaBF1tNoB5wlCeusgy",
		)
	})
}
