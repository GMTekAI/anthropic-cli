package cmd

import (
	"testing"

	"github.com/anthropics/anthropic-cli/v2/internal/mocktest"
)

func TestBetaOrganizationPluginsInstallationSettingsList(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--base-url", mocktest.ServerWithQuery(t, "beta=true"),
			"beta:organization:plugins:installation-settings", "list",
			"--max-items", "10",
			"--plugin-id", "plugin_id",
			"--limit", "1",
			"--organization-id", "organization_id",
			"--page", "page",
			"--target-type", "organization",
			"--beta", "message-batches-2024-09-24",
		)
	})
}

func TestBetaOrganizationPluginsInstallationSettingsRemove(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--base-url", mocktest.ServerWithQuery(t, "beta=true"),
			"beta:organization:plugins:installation-settings", "remove",
			"--plugin-id", "plugin_id",
			"--target", "target",
			"--beta", "message-batches-2024-09-24",
		)
	})
}

func TestBetaOrganizationPluginsInstallationSettingsSet(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--base-url", mocktest.ServerWithQuery(t, "beta=true"),
			"beta:organization:plugins:installation-settings", "set",
			"--plugin-id", "plugin_id",
			"--target", "target",
			"--installation-preference", "required",
			"--beta", "message-batches-2024-09-24",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("installation_preference: required")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--base-url", mocktest.ServerWithQuery(t, "beta=true"),
			"beta:organization:plugins:installation-settings", "set",
			"--plugin-id", "plugin_id",
			"--target", "target",
			"--beta", "message-batches-2024-09-24",
		)
	})
}
