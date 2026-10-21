package cmd

import (
	"testing"

	"github.com/anthropics/anthropic-cli/v2/internal/mocktest"
)

func TestBetaOrganizationSpendLimitsIncreaseRequestsRetrieve(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"beta:organization:spend-limits:increase-requests", "retrieve",
			"--spend-limit-increase-request-id", "spend_limit_increase_request_id",
		)
	})
}

func TestBetaOrganizationSpendLimitsIncreaseRequestsList(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"beta:organization:spend-limits:increase-requests", "list",
			"--max-items", "10",
			"--actor-id", "string",
			"--limit", "1",
			"--page", "page",
			"--status", "approved",
		)
	})
}

func TestBetaOrganizationSpendLimitsIncreaseRequestsApprove(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"beta:organization:spend-limits:increase-requests", "approve",
			"--spend-limit-increase-request-id", "spend_limit_increase_request_id",
			"--amount", "50000",
			"--period", "monthly",
			"--suppress-notification=true",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"amount: '50000'\n" +
			"period: monthly\n" +
			"suppress_notification: true\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"beta:organization:spend-limits:increase-requests", "approve",
			"--spend-limit-increase-request-id", "spend_limit_increase_request_id",
		)
	})
}

func TestBetaOrganizationSpendLimitsIncreaseRequestsDeny(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"beta:organization:spend-limits:increase-requests", "deny",
			"--spend-limit-increase-request-id", "spend_limit_increase_request_id",
			"--suppress-notification=true",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("suppress_notification: true")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"beta:organization:spend-limits:increase-requests", "deny",
			"--spend-limit-increase-request-id", "spend_limit_increase_request_id",
		)
	})
}
