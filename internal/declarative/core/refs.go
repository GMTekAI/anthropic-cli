package core

import (
	"fmt"
	"maps"
	"strings"
)

// RefSlot is a Ref located at a dotted path into the body. NewRegistry derives
// one for each field in KindSpec.Fields whose Ref is set. References are
// declared, never sniffed: sniffing would turn a system prompt that mentions
// "./notes" into a dependency.
type RefSlot struct {
	Path string
	Ref
}

// Target is a resolved reference: what the referenced resource became after
// (or, at plan time, is predicted to become after) apply.
type Target struct {
	Kind Kind
	ID   string
	// Version is the referenced resource's version in its JSON form: an
	// integer for some kinds, an epoch string for others. Nil when there is
	// none to pin yet.
	Version any
	// Pinned is false when the user asked to always track latest, in which
	// case EncodeObject omits the version and lets the server resolve it.
	Pinned bool
	// Versioned reports whether the target's kind has a version at all;
	// pinning is meaningless for one that does not.
	Versioned bool
	// Known is false at plan time for a resource that does not exist yet, so
	// its ID and version are "known after apply".
	Known bool
}

// KnownAfterApply is the placeholder a plan shows where a value will only
// exist once the resource it depends on has been created.
const KnownAfterApply = "(known after apply)"

func (t Target) idOrPlaceholder() any {
	if !t.Known {
		return KnownAfterApply
	}
	return t.ID
}

// versionOrPlaceholder renders the version to pin. A resource that is about to
// change has no version yet — the server assigns it — so the plan says so
// rather than quietly omitting the field, which would read as "unpinned".
func (t Target) versionOrPlaceholder() any {
	if t.Version != nil {
		return t.Version
	}
	return KnownAfterApply
}

// EncodeObject renders a reference as a discriminated object: a `type`, the ID
// under idField, and the pinned version when the target has one. This is the
// shape most APIs use for a typed reference; a domain names the discriminator
// and the field.
func EncodeObject(typeName, idField string) func(Target) any {
	return func(t Target) any {
		out := map[string]any{"type": typeName, idField: t.idOrPlaceholder()}
		if t.Pinned {
			out["version"] = t.versionOrPlaceholder()
		}
		return out
	}
}

// EncodeID renders a reference as a bare ID string.
func EncodeID(t Target) any { return t.idOrPlaceholder() }

// refExpr is one reference as written by the user, before resolution.
type refExpr struct {
	// Path is a working-tree path or glob, relative to the declaring file.
	Path string
	// URL is set instead of Path for URL-sourced resources.
	URL string
	// InlineID short-circuits resolution: the user gave a tagged ID directly.
	InlineID string
	// Pinned is false when the user asked for `version: latest`.
	Pinned bool
	// Passthrough is a fully specified API object the user wrote by hand, such
	// as a reference to something this config does not manage. It is copied
	// through untouched and creates no dependency edge.
	Passthrough any
	// Extra holds the keys written alongside `path` that are not reference
	// options: settings for this attachment rather than for the target. They
	// are merged into the encoded reference object.
	Extra map[string]any
}

func (r refExpr) isPassthrough() bool { return r.Passthrough != nil }

// parseRefValue interprets one entry in a reference slot.
func parseRefValue(r *Registry, v any) (refExpr, error) {
	switch t := v.(type) {
	case string:
		return parseRefString(r, t)
	case map[string]any:
		return parseRefObject(r, t)
	default:
		return refExpr{}, fmt.Errorf("expected a path, URL, ID or object, got %T", v)
	}
}

// parseRefObject interprets a reference written as an object. An object with a
// `path` key is a reference with options; anything else is a hand-written API
// object we pass straight through.
func parseRefObject(r *Registry, t map[string]any) (refExpr, error) {
	raw, hasPath := t["path"]
	if !hasPath {
		return refExpr{Passthrough: t}, nil
	}
	s, ok := raw.(string)
	if !ok {
		return refExpr{}, fmt.Errorf("`path` must be a string, got %T", raw)
	}
	expr, err := parseRefString(r, s)
	if err != nil {
		return refExpr{}, err
	}
	if version, ok := t["version"]; ok {
		if vs, _ := version.(string); !strings.EqualFold(vs, "latest") {
			return refExpr{}, fmt.Errorf("`version` alongside `path` may only be \"latest\"; to pin an exact version, reference the resource by ID instead")
		}
		expr.Pinned = false
	}
	extra := maps.Clone(t)
	delete(extra, "path")
	delete(extra, "version")
	if len(extra) > 0 {
		expr.Extra = extra
	}
	return expr, nil
}

func parseRefString(r *Registry, s string) (refExpr, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return refExpr{}, fmt.Errorf("empty reference")
	}
	expr := refExpr{Pinned: true}
	switch {
	case isURL(s):
		expr.URL = s
	case looksLikeID(r, s):
		expr.InlineID = s
	default:
		expr.Path = s
	}
	return expr, nil
}

// looksLikeID reports whether s is a tagged resource ID rather than a path or
// URL. A bare slug matching no registered prefix deliberately does not.
func looksLikeID(r *Registry, s string) bool {
	_, ok := r.KindForID(s)
	return ok
}
