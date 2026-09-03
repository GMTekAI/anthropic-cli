package claude

import (
	"net/url"
	"strings"

	"github.com/anthropics/anthropic-cli/internal/declarative/core"
)

// consolePages maps a kind to the Console section that shows it. Kinds with a
// detail page get the ID appended; skills only have a list.
var consolePages = map[core.Kind]struct {
	segment       string
	hasDetailPage bool
}{
	KindAgent:       {"agents", true},
	KindEnvironment: {"environments", true},
	KindDeployment:  {"deployments", true},
	KindMemoryStore: {"memory-stores", true},
	KindSkill:       {"skills", false},
}

// ConsoleLinks returns a function that maps a resource to its page in the
// Console, for the renderer to hang hyperlinks on, or nil when consoleURL is
// empty. workspaceID may be empty, in which case the Console's `default` alias
// is used.
func ConsoleLinks(consoleURL, workspaceID string) func(core.Kind, string) string {
	consoleURL = strings.TrimRight(consoleURL, "/")
	if consoleURL == "" {
		return nil
	}
	if workspaceID == "" {
		workspaceID = "default"
	}
	return func(kind core.Kind, id string) string {
		page, ok := consolePages[kind]
		if !ok || id == "" {
			return ""
		}
		link := consoleURL + "/workspaces/" + url.PathEscape(workspaceID) + "/" + page.segment
		if page.hasDetailPage {
			link += "/" + url.PathEscape(id)
		}
		return link
	}
}
