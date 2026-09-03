package claude

import (
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-cli/internal/declarative/core"
)

// prose says where a kind puts a markdown body. Every file-backed kind has one
// such place — an agent's system prompt, a deployment's first message, a
// store's description — so any of them can be written as `.md` (frontmatter
// for fields, body for the prose) or as `.yml` with the prose inline.
type prose struct {
	// field is the top-level field the body stands in for; declaring both is
	// contradictory rather than additive.
	field string
	// set writes the body into the request. Nil means fields[field] = body.
	set func(fields map[string]any, body string)
}

// build returns the Build function for a kind: the declared fields are the
// request body, a markdown body (if any) goes where p says, and a resource
// with no `name` takes the filename so a minimal file is a legal one.
func build(p prose) func(*core.Candidate) (map[string]any, error) {
	return func(c *core.Candidate) (map[string]any, error) {
		if body := strings.TrimSpace(string(c.Prose)); body != "" {
			if _, ok := c.Fields[p.field]; ok {
				return nil, fmt.Errorf(
					"%s: has a markdown body and also sets `%s`, which is what the body becomes in a .md file; keep one",
					c.Path, p.field)
			}
			if p.set != nil {
				p.set(c.Fields, body)
			} else {
				c.Fields[p.field] = body
			}
		}
		if _, ok := c.Fields["name"]; !ok {
			c.Fields["name"] = c.Name
		}
		return c.Fields, nil
	}
}

// firstMessage is a deployment's prose: the body becomes the user message
// every session it starts opens with.
var firstMessage = prose{
	field: "initial_events",
	set: func(fields map[string]any, body string) {
		fields["initial_events"] = []any{map[string]any{
			"type":    "user.message",
			"content": []any{map[string]any{"type": "text", "text": body}},
		}}
	},
}
