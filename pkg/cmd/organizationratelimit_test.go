package cmd

import (
	"testing"

	"github.com/anthropics/anthropic-cli/v2/internal/mocktest"
)

func TestOrganizationRateLimitsList(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"organization:rate-limits", "list",
			"--max-items", "10",
			"--group-type", "batch",
			"--limit", "1",
			"--model", "model",
			"--page", "page",
		)
	})
}
