package cmd

import (
	"testing"

	"github.com/anthropics/anthropic-cli/internal/mocktest"
	"github.com/anthropics/anthropic-cli/internal/requestflag"
)

func TestUserProfilesActionURLsCreate(t *testing.T) {
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"user-profiles:action-urls", "create",
			"--user-profile-id", "uprof_011CZkZCu8hGbp5mYRQgUmz9",
			"--action", "{type: enroll_end_user}",
			"--workspace-id", "wrkspc_011CZkZaBF1tNoB5wlCeusgy",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(userProfilesActionURLsCreate)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"--api-key", "string",
			"user-profiles:action-urls", "create",
			"--user-profile-id", "uprof_011CZkZCu8hGbp5mYRQgUmz9",
			"--action.type", "enroll_end_user",
			"--workspace-id", "wrkspc_011CZkZaBF1tNoB5wlCeusgy",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"action:\n" +
			"  type: enroll_end_user\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"--api-key", "string",
			"user-profiles:action-urls", "create",
			"--user-profile-id", "uprof_011CZkZCu8hGbp5mYRQgUmz9",
			"--workspace-id", "wrkspc_011CZkZaBF1tNoB5wlCeusgy",
		)
	})
}
