package cmd

import (
	"testing"

	"github.com/anthropics/anthropic-cli/v2/internal/mocktest"
)

func TestBetaOrganizationAnalyticsSkillsList(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--base-url", mocktest.ServerWithQuery(t, "beta=true"),
			"beta:organization:analytics:skills", "list",
			"--max-items", "10",
			"--date", "'2019-12-27'",
			"--ending-date", "'2019-12-27'",
			"--filter", "string",
			"--group-by", "product",
			"--limit", "1",
			"--order", "asc",
			"--order-by", "order_by",
			"--page", "page",
			"--starting-date", "'2019-12-27'",
		)
	})
}
