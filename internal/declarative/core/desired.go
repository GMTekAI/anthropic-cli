package core

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// desiredBody is a fully resolved request body plus everything the plan needs
// to describe it honestly.
type desiredBody struct {
	Body map[string]any
	// Sensitive lists dotted paths whose values must never be printed.
	Sensitive map[string]bool
	// Unresolved is true when the body embeds a "(known after apply)"
	// placeholder, meaning a dependency does not exist yet.
	Unresolved bool
}

// bodyBuilder renders request bodies. It holds the resolved slots outright so
// that apply — which must rebuild every body once its dependencies have real
// IDs — needs neither the Planner nor the Loader.
type bodyBuilder struct {
	registry *Registry
	slots    map[string][]resolvedSlot
}

// build renders the request body for a source, resolving every reference
// against the targets computed so far.
func (b bodyBuilder) build(src *Source, resolved map[string]Target) (*desiredBody, error) {
	out := &desiredBody{
		Body:      deepCopyMap(src.Body),
		Sensitive: map[string]bool{},
	}
	spec := b.registry.specOrZero(src.Kind)

	for _, slot := range b.slots[src.Key] {
		if err := applySlot(src, slot, resolved, out); err != nil {
			return nil, err
		}
	}

	for _, path := range spec.writeOnly {
		out.Sensitive[path] = true
	}
	return out, nil
}

// applySlot writes one slot's references into the body as the API takes them.
func applySlot(src *Source, slot resolvedSlot, resolved map[string]Target, out *desiredBody) error {
	values := make([]any, 0, len(slot.Entries))
	for _, entry := range slot.Entries {
		if entry.TargetKey == "" {
			values = append(values, entry.Value)
			continue
		}
		target, ok := resolved[entry.TargetKey]
		if !ok {
			return fmt.Errorf("%s: internal error: %s was not resolved before its dependent", src.Key, entry.TargetKey)
		}
		// Pinning only means anything for a kind that carries a version; a
		// reference to any other kind is just an ID.
		target.Pinned = entry.Pinned && target.Versioned
		if !target.Known || (target.Pinned && target.Version == nil) {
			out.Unresolved = true
		}
		value, err := withExtra(slot.Ref.encode(target), entry.Extra)
		if err != nil {
			return fmt.Errorf("%s: `%s`: %w", src.Key, slot.Ref.Path, err)
		}
		values = append(values, value)
	}

	if slot.Ref.List {
		setPath(out.Body, slot.Ref.Path, values)
		return nil
	}
	if len(values) != 1 {
		return fmt.Errorf("%s: `%s` resolved to %d values but takes exactly one", src.Key, slot.Ref.Path, len(values))
	}
	setPath(out.Body, slot.Ref.Path, values[0])
	return nil
}

// withExtra merges the keys written alongside `path` into an encoded
// reference. They may add to the object but not contradict what the
// reference itself resolved to, and a reference that encodes as a bare ID has
// nowhere to put them.
func withExtra(encoded any, extra map[string]any) (any, error) {
	if len(extra) == 0 {
		return encoded, nil
	}
	obj, ok := encoded.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("this reference is sent as a bare ID, so it cannot carry extra keys (%s)", strings.Join(slices.Sorted(maps.Keys(extra)), ", "))
	}
	for k, v := range extra {
		if _, taken := obj[k]; taken {
			return nil, fmt.Errorf("`%s` is filled in from `path` and cannot also be set by hand", k)
		}
		obj[k] = v
	}
	return obj, nil
}
