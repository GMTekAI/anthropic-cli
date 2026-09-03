package claude

import (
	"testing"

	"github.com/anthropics/anthropic-cli/internal/declarative/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A lockfile written against one API host or organization must not be planned
// with credentials for another: every read would 404 and the plan would offer
// to recreate everything under new ids.
func TestCheckOriginRefusesADifferentHostOrOrganization(t *testing.T) {
	recorded := core.Origin{BaseURL: "https://api.example.com", OrganizationID: "org-a", WorkspaceID: "wrkspc-1"}
	lock := &core.Lockfile{Path: "claude-lock.json", Origin: &recorded, Resources: map[string]*core.LockEntry{
		"./agents/a.md": {Kind: KindAgent, ID: "agent_01AQG5Ht2PgY2L1Ss9e829AP"},
	}}

	assert.NoError(t, CheckOrigin(lock, recorded))
	assert.NoError(t, CheckOrigin(lock, core.Origin{BaseURL: "https://API.example.com/"}), "an unknown org is not a mismatch, and hosts compare loosely")

	err := CheckOrigin(lock, core.Origin{BaseURL: "https://api.anthropic.com", OrganizationID: "org-a"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api.example.com")

	err = CheckOrigin(lock, core.Origin{BaseURL: "https://api.example.com", OrganizationID: "org-b"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "organization")

	err = CheckOrigin(lock, core.Origin{BaseURL: "https://api.example.com", OrganizationID: "org-a", WorkspaceID: "wrkspc-2"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace")

	// An empty lockfile, or one that never recorded an origin, goes anywhere.
	assert.NoError(t, CheckOrigin(&core.Lockfile{}, core.Origin{BaseURL: "https://api.anthropic.com"}))
	lock.Origin = nil
	assert.NoError(t, CheckOrigin(lock, core.Origin{BaseURL: "https://api.anthropic.com"}))
}
