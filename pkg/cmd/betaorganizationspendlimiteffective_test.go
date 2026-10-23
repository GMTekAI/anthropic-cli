package cmd

import (
	"testing"

	"github.com/anthropics/anthropic-cli/v2/internal/mocktest"
)

func TestBetaOrganizationSpendLimitsEffectiveList(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--base-url", mocktest.ServerWithQuery(t, "beta=true"),
			"beta:organization:spend-limits:effective", "list",
			"--max-items", "10",
			"--limit", "1",
			"--page", "page",
			"--period", "daily",
			"--user-id", "string",
		)
	})
}
