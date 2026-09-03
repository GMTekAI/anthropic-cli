// Package conformance is a tool-neutral format for reconcile test cases, and a
// runner that drives any declarative client of the Claude Developer Platform
// through them against the real service.
//
// It exists so that `ant apply` and the Terraform provider — two tools that
// turn files into the same API calls — can share the cases that pin down what
// "converged" means: a create then an unchanged re-plan is empty, editing a
// sub-agent re-pins its coordinator, a field deleted from a file is cleared on
// the server, a server-normalised value is not drift. Nothing in this package
// imports either tool; each supplies an Adapter that renders desired state in
// its own language and reports its plan in the vocabulary below.
//
// # Case files
//
// A case is one YAML file. Desired state is written as API create bodies keyed
// by a logical name, so neither tool's configuration language is privileged:
//
//	schema: 1
//	name: editing a sub-agent re-pins its coordinator
//	resources:
//	  helper:
//	    kind: agent
//	    body: {name: "cf-{run}-helper", model: claude-sonnet-4-5, system: Help carefully.}
//	  lead:
//	    kind: agent
//	    body:
//	      name: "cf-{run}-lead"
//	      model: claude-sonnet-4-5
//	      multiagent: {type: coordinator, agents: [{$ref: helper}]}
//	steps:
//	  - expect:
//	      plan:   {helper: create, lead: create}
//	      remote: {lead: {"multiagent.agents[0].version": {$version: helper}}}
//	  - change: {helper: {body: {system: Help thoroughly.}}}
//	    expect:
//	      plan:   {helper: {update: [system]}, lead: {update: [multiagent]}}
//	  - remote: {helper: archive}
//	    expect: {error: "archived|no longer"}
//	  - flags: [force]
//	    expect:
//	      plan:   {helper: replace, lead: {update: [multiagent]}}
//
// The rules:
//
//   - {run} is replaced everywhere (bodies and file contents) with a short id
//     unique to the run, and every resource's name must contain it: cases run
//     end to end in a shared organization, in parallel, and some names (skill
//     titles) are unique per organization.
//   - A reference to another resource is written {$ref: name}. It stands for
//     "however this tool points at that resource in this slot" — a relative
//     path for ant, an attribute reference for Terraform — so cases never
//     contain literal ids.
//   - Skills carry their content as files: {SKILL.md: "...", notes.md: "..."},
//     paths relative to the skill's root directory.
//   - The first step's desired state is `resources`. A later step's `change` is
//     a JSON merge patch per resource (RFC 7396: objects merge, null deletes,
//     anything else replaces); `name: null` removes the resource, and a new
//     name with a `kind` adds one. Steps therefore read as what changed.
//   - `remote` performs out-of-band edits before the step through the API
//     directly, never through the tool under test: `archive`/`delete`, or
//     {patch: {body...}, files: {...}} for a Console edit or a version
//     published elsewhere.
//   - `flags` is a closed list — force, prune — that each adapter maps to its
//     own spelling or ignores if it has no equivalent.
//   - `expect.plan` maps names to create | noop | update | {update: [top-level
//     fields]} | replace | destroy. Names not listed must plan as noop, so an
//     unexpected cascade fails the case. Omit `plan` to skip the check.
//   - `expect.error` is a regular expression the plan (or, if the plan
//     succeeds, the apply) must fail with. The step then checks nothing else:
//     no `expect.plan`, no re-plan, no `expect.remote`.
//   - `expect.remote` reads each named resource back from the API and checks
//     dotted paths (`a.b[0].c`) against literals or matchers: {$id: name},
//     {$version: name}, {$set: true}, {$unset: true}, {$match: regex}.
//   - After every step that applies, the runner re-plans and requires an empty
//     plan. When the case ends, whether it passed or failed, the runner has the
//     adapter destroy everything and confirms through the API that each
//     resource is gone or archived.
//   - `skip: {tool: reason}` and `only: [tool]` are the only per-tool escape
//     hatches. A case that would need different input per tool is not a shared
//     case; keep it in that tool's own suite.
//   - A case that declares a kind the adapter does not manage is skipped for
//     that adapter.
//
// # Running
//
// Load reads a directory of cases; Run drives them through an Adapter, using a
// Remote for the out-of-band edits and read-backs. Both talk to the real API,
// so callers gate Run behind whatever opt-in their acceptance tests use.
package conformance
