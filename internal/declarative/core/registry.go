package core

import (
	"slices"
	"strings"
)

// Schema is how a domain describes its file layout. Core reads files, resolves
// references between them and reconciles the result; it has no opinion about
// which file means what, so every convention — a directory name, an extension,
// a marker file — lives behind this interface rather than as a flag core has to
// know the meaning of.
type Schema interface {
	// Classify names the kind a parsed file declares. Returning ("", nil) means
	// "not a resource": a directory walk must tolerate that, since a repository
	// is full of prose and CI config. walked reports whether the file was found
	// by walking rather than named outright, which is usually the difference
	// between skipping something and complaining about it. Classify may read
	// the `type` field to decide; core drops that field from c.Fields
	// afterwards, so a Build never sees it.
	Classify(c *Candidate, walked bool) (Kind, error)
	// IsResourceDir reports whether a directory is itself a resource rather
	// than a folder of them. It is asked often, so it must be cheap — a stat,
	// not a walk.
	IsResourceDir(dir string) (Kind, bool)
	// LoadDir loads a directory IsResourceDir accepted.
	LoadDir(dir string) (Kind, map[string]any, Payload, error)
	// ResourceDirOf maps a file that stands in for a directory-backed resource
	// to that directory — a marker file to its folder.
	ResourceDirOf(path string) (string, bool)
	// LockfileName is the state file's conventional name. A directory walk
	// skips it, and discovery looks for it walking up.
	LockfileName() string
}

// Registry is the set of kinds a reconciler works with, in an order where a
// kind's dependencies come before it.
type Registry struct {
	schema Schema
	order  []Kind
	specs  map[Kind]KindSpec
}

// NewRegistry builds a registry. Specs must be given dependencies-first: plan
// output, tie-breaking and destroy ordering all rely on it, and
// TestKindOrderIsTopological in the domain package is what keeps it honest.
func NewRegistry(schema Schema, specs ...KindSpec) *Registry {
	r := &Registry{schema: schema, specs: make(map[Kind]KindSpec, len(specs))}
	for _, s := range specs {
		// Flatten the field map once, so the reconciler's hot paths stay simple
		// lists rather than re-scanning the field map on every comparison.
		s.computed = s.Fields.paths(func(f Field) bool { return f.Computed })
		s.writeOnly = s.Fields.paths(func(f Field) bool { return f.WriteOnly })
		s.immutable = s.Fields.paths(func(f Field) bool { return f.Immutable })
		s.clearable = s.Fields.paths(func(f Field) bool { return f.Clearable })
		s.summary = s.Fields.paths(func(f Field) bool { return f.Summary })
		s.refs = s.Fields.refSlots()
		s.includes = s.Fields.paths(func(f Field) bool { return f.Include })
		s.shorthand = map[string]string{}
		s.matchBy = map[string][]string{}
		for path, f := range s.Fields {
			if f.Shorthand != "" {
				s.shorthand[path] = f.Shorthand
			}
			if f.MatchBy != nil {
				s.matchBy[path] = f.MatchBy
			}
			if f.Archived {
				s.archivedField = path
			}
			if f.Metadata {
				s.metadataField = path
			}
		}
		// A kind whose content is a payload rather than a body answers writes
		// with a version object; the applier needs to know which those are.
		s.payloadBacked = s.Build == nil
		r.order = append(r.order, s.Kind)
		r.specs[s.Kind] = s
	}
	return r
}

// Schema is the domain's description of its file layout.
func (r *Registry) Schema() Schema { return r.schema }

// LockfileName is the state file's conventional name, as the domain has it.
func (r *Registry) LockfileName() string { return r.schema.LockfileName() }

func (r *Registry) isLockfileName(name string) bool {
	return strings.EqualFold(name, r.LockfileName())
}

// Kinds lists every kind, dependencies first.
func (r *Registry) Kinds() []Kind { return r.order }

// Spec returns a kind's specification.
func (r *Registry) Spec(k Kind) (KindSpec, bool) {
	s, ok := r.specs[k]
	return s, ok
}

// specOrZero returns a kind's specification, or a zero value. Internal callers
// have already validated the kind; this keeps them from threading an error
// through every accessor.
func (r *Registry) specOrZero(k Kind) KindSpec { return r.specs[k] }

// Valid reports whether a kind is registered.
func (r *Registry) Valid(k Kind) bool {
	_, ok := r.specs[k]
	return ok
}

// rank is a kind's position in dependency order.
func (r *Registry) rank(k Kind) int {
	if i := slices.Index(r.order, k); i >= 0 {
		return i
	}
	return len(r.order)
}

// idSuffixLen is the length of a tagged ID's final segment: the 24
// alphanumerics after the prefix, or after the label when there is one. Holding
// references to that shape is what stops `agent_defs/verifier.md` being sent as
// an agent ID.
const idSuffixLen = 24

// KindForID maps a tagged ID back to its kind. It returns false for anything
// that does not look like an ID, which is how a reference is told apart from a
// file path. Too strict fails loudly (an ID treated as a path matches nothing
// on disk); too loose fails silently (a path sent to the API as an ID).
//
// An ID may carry one lowercase label between the prefix and the suffix
// ("agent_label_0123…"); that is still an ID.
func (r *Registry) KindForID(id string) (Kind, bool) {
	prefix, rest, ok := strings.Cut(id, "_")
	if !ok {
		return "", false
	}
	if label, suffix, hasLabel := strings.Cut(rest, "_"); hasLabel {
		if label == "" || !isLower(label) {
			return "", false
		}
		rest = suffix
	}
	if len(rest) != idSuffixLen || !isAlnum(rest) {
		return "", false
	}
	for _, k := range r.order {
		if r.specs[k].IDPrefix == prefix {
			return k, true
		}
	}
	return "", false
}

func isAlnum(s string) bool {
	for _, c := range s {
		if !('0' <= c && c <= '9' || 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z') {
			return false
		}
	}
	return true
}

func isLower(s string) bool {
	for _, c := range s {
		if !('a' <= c && c <= 'z') {
			return false
		}
	}
	return true
}
