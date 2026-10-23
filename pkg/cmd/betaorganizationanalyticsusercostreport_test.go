package cmd

import (
	"testing"

	"github.com/anthropics/anthropic-cli/v2/internal/mocktest"
)

func TestBetaOrganizationAnalyticsUserCostReportList(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--base-url", mocktest.ServerWithQuery(t, "beta=true"),
			"beta:organization:analytics:user-cost-report", "list",
			"--max-items", "10",
			"--starting-at", "'2019-12-27T18:11:19.117Z'",
			"--bucket-width", "1d",
			"--claude-tag-category", "engaged",
			"--claude-tag-user-id", "U0123ABCDEF",
			"--context-window", "0-200k",
			"--ending-at", "'2019-12-27T18:11:19.117Z'",
			"--exclude-deleted-users=true",
			"--group-by", "claude_tag_category",
			"--inference-geo", "global",
			"--limit", "1",
			"--model", "string",
			"--order", "asc",
			"--order-by", "amount",
			"--page", "page",
			"--product", "chat",
			"--rbac-group-id", "rbac_group_012rppKaSVsmTo6NqRDXQXNF",
			"--slack-channel-id", "C0123ABCDEF",
			"--speed", "fast",
			"--user-id", "user_01AbCdEfGhIjKlMnOpQrSt",
		)
	})
}
