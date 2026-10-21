package cmd

import (
	"testing"

	"github.com/anthropics/anthropic-cli/v2/internal/mocktest"
)

func TestOrganizationAPIKeysRetrieve(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"organization:api-keys", "retrieve",
			"--api-key-id", "api_key_id",
		)
	})
}

func TestOrganizationAPIKeysUpdate(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"organization:api-keys", "update",
			"--api-key-id", "api_key_id",
			"--name", "x",
			"--status", "active",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"name: x\n" +
			"status: active\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"organization:api-keys", "update",
			"--api-key-id", "api_key_id",
		)
	})
}

func TestOrganizationAPIKeysList(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"organization:api-keys", "list",
			"--max-items", "10",
			"--after-id", "after_id",
			"--before-id", "before_id",
			"--created-by-user-id", "created_by_user_id",
			"--limit", "1",
			"--status", "active",
			"--workspace-id", "workspace_id",
		)
	})
}
