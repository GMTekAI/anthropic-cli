package cmd

import (
	"testing"

	"github.com/anthropics/anthropic-cli/v2/internal/mocktest"
)

func TestOrganizationComplianceSettingsRetrieve(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"organization:compliance-settings", "retrieve",
		)
	})
}

func TestOrganizationComplianceSettingsUpdate(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"organization:compliance-settings", "update",
			"--state", "{type: enabled}",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"state:\n" +
			"  type: enabled\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"organization:compliance-settings", "update",
		)
	})
}
