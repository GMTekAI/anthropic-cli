package claude

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConsoleLinks(t *testing.T) {
	link := ConsoleLinks("https://platform.claude.com/", "wrkspc_01abc")
	assert.Equal(t, "https://platform.claude.com/workspaces/wrkspc_01abc/agents/agent_01x", link(KindAgent, "agent_01x"))
	assert.Equal(t, "https://platform.claude.com/workspaces/wrkspc_01abc/deployments/depl_01x", link(KindDeployment, "depl_01x"))
	assert.Equal(t, "https://platform.claude.com/workspaces/wrkspc_01abc/memory-stores/memstore_01x", link(KindMemoryStore, "memstore_01x"))
	assert.Equal(t, "https://platform.claude.com/workspaces/wrkspc_01abc/skills", link(KindSkill, "skill_01x"),
		"skills have no detail page, so the link lands on the list")

	assert.Equal(t, "https://platform.claude.com/workspaces/default/agents/agent_01x",
		ConsoleLinks("https://platform.claude.com", "")(KindAgent, "agent_01x"),
		"no workspace falls back to the Console's default alias")
	assert.Nil(t, ConsoleLinks("", "wrkspc_01abc"), "no Console, no links")
}
