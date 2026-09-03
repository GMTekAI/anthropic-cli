// Package claude teaches the core reconciler about the Claude Developer
// Platform: which resources exist, what their files look like, which fields
// the server owns, and how to talk to the API.
//
// Everything Claude-specific in `ant apply` lives here. The core package holds
// the reconciler itself and does not import this one, so a fact about the API
// has exactly one home — and when the spec moves, this is the only directory
// to re-read.
package claude

import (
	"maps"

	"github.com/anthropics/anthropic-cli/internal/declarative/core"
)

// The kinds `ant apply` manages.
const (
	KindSkill       core.Kind = "skill"
	KindEnvironment core.Kind = "environment"
	KindMemoryStore core.Kind = "memory_store"
	KindAgent       core.Kind = "agent"
	KindDeployment  core.Kind = "deployment"
)

// Removal is not uniform: a skill can only be deleted, and every other kind is
// archived. Where the API offers both we archive, because an archived resource
// keeps its data and a deleted one does not.
var (
	byArchiving = core.Destroy{Verb: "archive", Past: "archived"}
	byDeleting  = core.Destroy{Verb: "delete", Past: "deleted"}
)

// Field sets shared across kinds, so each kind's own Fields list only what is
// its own.
var (
	// identity is on every resource: the server's fields, and the name a
	// plan shows for it.
	identity = core.Fields{
		"id":         {Computed: true},
		"type":       {Computed: true},
		"created_at": {Computed: true},
		"updated_at": {Computed: true},
		"name":       {Summary: true},
	}
	// archivable kinds report when they were archived; there is no unarchive.
	archivable = core.Fields{
		"archived_at": {Computed: true, Archived: true},
	}
	// metadata is the user key-value bag, patched key by key.
	metadata = core.Fields{
		"metadata": {Metadata: true},
	}
)

// computed marks each named field as set by the server.
func computed(names ...string) core.Fields {
	f := core.Fields{}
	for _, name := range names {
		f[name] = core.Field{Computed: true}
	}
	return f
}

// with merges field sets into one; a later set wins on a shared path.
func with(sets ...core.Fields) core.Fields {
	out := core.Fields{}
	for _, set := range sets {
		maps.Copy(out, set)
	}
	return out
}

// Registry describes every kind to the reconciler.
//
// The Fields below are hand-transcribed from the API specification, and
// nothing enforces that they stay in step with it. When the SDK is bumped,
// re-check each one against the endpoint's params and response structs:
//
//   - a new server-computed response field not marked Computed causes
//     permanent phantom drift, blocking every plan;
//   - a new secret not marked WriteOnly gets printed in the plan;
//   - a new clearable field not marked Clearable means deleting the line
//     from a file silently does nothing.
//
// None of these show up at build time.
//
// Order matters: a kind must come after everything it can reference.
// TestKindOrderIsTopological holds that.
func Registry() *core.Registry {
	return core.NewRegistry(schema{},
		core.KindSpec{
			Kind:     KindSkill,
			IDPrefix: "skill",
			// A skill's content is its files; the server derives name and
			// description from SKILL.md and mints a version per upload.
			VersionField: "latest_version_id",
			Destroy:      byDeleting,
			Fields: with(identity, computed("latest_version_id", "source"), core.Fields{
				// Skills have no update endpoint and a version upload carries
				// only files, so a title change cannot be applied at all; and
				// the `name` in SKILL.md is the skill's identity, which a new
				// version may not change.
				"display_name": {Immutable: true, Summary: true},
				"name":         {Immutable: true, Summary: true},
			}),
		},
		core.KindSpec{
			Kind:     KindEnvironment,
			IDPrefix: "env",
			Destroy:  byArchiving,
			Build:    build(prose{field: "description"}),
			Fields: with(identity, archivable, metadata, core.Fields{
				"description": {Clearable: true},
			}),
		},
		core.KindSpec{
			// The store is the container; the memories inside it are the
			// agent's runtime data and are never read, diffed or removed here.
			Kind:     KindMemoryStore,
			IDPrefix: "memstore",
			Destroy:  byArchiving,
			Build:    build(prose{field: "description"}),
			Fields: with(identity, archivable, metadata, core.Fields{
				// Rendered into the system prompt of every session the store is
				// attached to, so it is worth showing in the plan.
				"description": {Clearable: true, Summary: true},
			}),
		},
		core.KindSpec{
			Kind:               KindAgent,
			IDPrefix:           "agent",
			VersionField:       "version",
			VersionIsInt:       true,
			UpdateNeedsVersion: true,
			Destroy:            byArchiving,
			Build:              build(prose{field: "system"}),
			Fields: with(identity, archivable, metadata, computed("version"), core.Fields{
				// `model: x` is accepted, but read back as `{id: x, speed: …}`.
				"model":       {Shorthand: "id", Summary: true},
				"description": {Clearable: true},
				"system":      {Clearable: true},
				// A plan pairs list elements up before diffing them; these are
				// the members that say which element is which. Both lists also
				// take `- ./path.yml` entries spliced in from a data file, for
				// tool specs a framework generates rather than a person types.
				"tools":       {Clearable: true, MatchBy: []string{"name", "type"}, Include: true},
				"mcp_servers": {Clearable: true, MatchBy: []string{"name", "url"}, Include: true},
				"multiagent":  {Clearable: true},
				"skills": {
					Clearable: true,
					MatchBy:   []string{"skill_id"},
					Ref:       &core.Ref{To: []core.Kind{KindSkill}, List: true, As: core.EncodeObject("custom", "skill_id")},
				},
				"multiagent.agents": {
					Ref: &core.Ref{To: []core.Kind{KindAgent}, List: true, As: core.EncodeObject("agent", "id")},
				},
			}),
		},
		core.KindSpec{
			Kind:     KindDeployment,
			IDPrefix: "depl",
			Destroy:  byArchiving,
			Build:    build(firstMessage),
			Fields: with(identity, archivable, metadata, computed("status", "paused_reason"), core.Fields{
				"description": {Clearable: true},
				// A mixed list: files and repositories are written as the API
				// takes them and pass through; a memory store may be a path to
				// its file, optionally with `access` and `instructions` alongside.
				"resources": {
					Clearable: true,
					MatchBy:   []string{"memory_store_id", "url", "type"},
					Ref:       &core.Ref{To: []core.Kind{KindMemoryStore}, List: true, As: core.EncodeObject("memory_store", "memory_store_id")},
				},
				"schedule":                        {Summary: true},
				"schedule.last_run_at":            {Computed: true},
				"schedule.upcoming_runs_at":       {Computed: true},
				"resources[].authorization_token": {WriteOnly: true},
				"agent": {
					Summary: true,
					Ref:     &core.Ref{To: []core.Kind{KindAgent}, As: core.EncodeObject("agent", "id")},
				},
				"environment_id": {
					Summary: true,
					Ref:     &core.Ref{To: []core.Kind{KindEnvironment}, As: core.EncodeID},
				},
			}),
		},
	)
}
