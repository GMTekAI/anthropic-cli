// Package core reconciles a tree of declarative files against a remote API:
// it parses them, resolves the references between them, orders the work, diffs
// it against what the server holds, and records the outcome in a lockfile.
//
// It knows nothing about any particular API beyond its general shape: JSON
// bodies, tagged string IDs, optional versions. Everything domain-specific —
// what kinds exist, which fields the server owns, what the files and lockfile
// are called, how to talk to it — arrives through a Registry and a Client. The
// sibling claude package is the implementation for the Claude Developer Platform.
//
// A run is Loader → Planner → Plan → Applier, with the Lockfile carrying state
// between runs. The files, in that order:
//
//	spec.go      KindSpec and Field: what a domain says about each kind
//	registry.go  Registry and Schema: the set of kinds, and the file layout
//	client.go    Client, Request and Payload: the API a domain provides
//	source.go    Source and Candidate: one file read and decoded
//	loader.go    Loader: paths to a closed set of Sources
//	resolve.go   following the references a Source makes
//	refs.go      reference syntax and Target, what a reference resolves to
//	include.go   splicing data files into list fields at load time
//	fetch.go     URLFetcher and the GitHub implementation
//	graph.go     dependency order and cycle reporting
//	desired.go   rendering the request body with references filled in
//	plan.go      Planner and Plan: what apply would do, and why
//	diff.go      field-level Diff, drift comparison, redaction
//	version.go   versions from the server, into the lockfile, and back out
//	apply.go     Applier: executing a Plan, saving state after each write
//	lock.go      Lockfile: the committed state file
//	hash.go      canonical JSON and fingerprints
//	bodypath.go  dotted-path helpers over decoded JSON
package core
