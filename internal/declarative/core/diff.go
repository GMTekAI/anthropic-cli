package core

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Comparing what the files ask for against what the server holds.
//
// The decision to update lives in plan.go and is made from hashes; everything
// here answers the follow-up question — *what* differs, how to say so without
// disclosing a secret, and which differences the API simply cannot carry out.

// Diff describes how the desired body differs from the server's copy, as a
// tree that follows the body's own shape. A nil *Diff means no difference.
type Diff struct {
	Kind DiffKind
	// Fields holds the differing members of an object, by name.
	Fields map[string]*Diff
	// Items holds the differing elements of a list, in display order.
	Items []ItemDiff
	// Before is the server's value and After the desired one, for the leaf
	// kinds that carry them.
	Before, After any
}

// DiffKind says what sort of node a Diff is.
type DiffKind int

const (
	// DiffObject and DiffList are interior nodes; see Fields and Items.
	DiffObject DiffKind = iota + 1
	DiffList
	// DiffValue is a changed leaf: Before became After.
	DiffValue
	// DiffText is a changed string, kept apart so a renderer can diff words.
	DiffText
	// DiffAdded is a value the server does not have yet (After only).
	DiffAdded
	// DiffRemoved is a value being cleared or a list element being dropped
	// (Before only).
	DiffRemoved
	// DiffSensitive is a change whose values must not be shown.
	DiffSensitive
	// DiffWriteOnly is a field the server never returns: it will be sent
	// again, and whether that changes anything is unknowable.
	DiffWriteOnly
)

// ItemDiff is one element of a list diff. Before and After are the element's
// index on the server and in the desired body; -1 means absent on that side.
// A nil Diff with both indices set is an element that only moved.
type ItemDiff struct {
	Before, After int
	Diff          *Diff
}

// defaultMatchBy is how list elements are paired up when the field declares
// nothing better: by id, then by name, then by position.
var defaultMatchBy = []string{"id", "name"}

// buildDiff produces the field-level diff shown under an update. The decision
// to update is made from hashes; this is presentation, and it deliberately
// ignores extra keys the server filled in — except for a clearable field the
// file dropped, which apply will null out and the plan must therefore show.
func (c *Change) buildDiff() *Diff {
	if c.Remote == nil {
		return nil
	}
	d := c.diffObject("", c.Desired, c.Remote)
	for _, k := range c.spec.clearable {
		if _, declared := c.Desired[k]; declared {
			continue
		}
		got, present := c.Remote[k]
		if !present || IsEmptyValue(got) {
			continue
		}
		d = orEmptyObject(d)
		d.Fields[k] = c.redact(k, &Diff{Kind: DiffRemoved, Before: got})
	}
	// Likewise the metadata bag: the keys apply will null out are removals.
	if f := c.spec.metadataField; f != "" {
		want, _ := c.Desired[f].(map[string]any)
		got, _ := c.Remote[f].(map[string]any)
		for _, k := range removedMetadataKeys(want, got) {
			d = orEmptyObject(d)
			bag := orEmptyObject(d.Fields[f])
			d.Fields[f] = bag
			if bag.Kind != DiffObject {
				break // the whole bag is already shown as added or removed
			}
			bag.Fields[k] = &Diff{Kind: DiffRemoved, Before: got[k]}
		}
	}
	return d
}

// orEmptyObject lets a removal be recorded on a diff that found nothing else.
func orEmptyObject(d *Diff) *Diff {
	if d == nil {
		return &Diff{Kind: DiffObject, Fields: map[string]*Diff{}}
	}
	return d
}

func (c *Change) diffObject(path string, want, got map[string]any) *Diff {
	fields := map[string]*Diff{}
	for k, wv := range want {
		gv, present := got[k]
		if child := c.diffAny(joinPath(path, k), wv, gv, present); child != nil {
			fields[k] = child
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return &Diff{Kind: DiffObject, Fields: fields}
}

func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

// diffAny compares one desired value against the server's. present says
// whether the server returned the field at all, which is not the same as
// returning null.
func (c *Change) diffAny(path string, want, got any, present bool) *Diff {
	if !present && slices.Contains(c.spec.writeOnly, path) {
		return &Diff{Kind: DiffWriteOnly}
	}
	// A shorthand field is written as a scalar and read back as an object;
	// compare against the member the scalar stands for.
	if sub, ok := c.spec.shorthand[path]; ok {
		if _, isMap := want.(map[string]any); !isMap {
			if g, ok := got.(map[string]any); ok {
				got, present = g[sub]
			}
		}
	}
	if !present {
		if IsEmptyValue(want) {
			return nil
		}
		return c.redact(path, &Diff{Kind: DiffAdded, After: want})
	}
	if equivalent(want, got) {
		return nil
	}
	if c.declaredSensitive(path) {
		return &Diff{Kind: DiffSensitive}
	}
	switch w := want.(type) {
	case map[string]any:
		if g, ok := got.(map[string]any); ok {
			return c.diffObject(path, w, g)
		}
	case []any:
		if g, ok := got.([]any); ok {
			return c.diffList(path, w, g)
		}
	case string:
		if g, ok := got.(string); ok {
			return &Diff{Kind: DiffText, Before: g, After: w}
		}
	}
	if IsEmptyValue(want) {
		return c.redact(path, &Diff{Kind: DiffRemoved, Before: got})
	}
	return c.redact(path, &Diff{Kind: DiffValue, Before: got, After: want})
}

// diffList pairs desired elements with the server's before comparing them, so
// inserting one element reads as one addition rather than every later element
// having changed.
func (c *Change) diffList(path string, want, got []any) *Diff {
	matchBy := c.spec.matchBy[path]
	if matchBy == nil {
		matchBy = defaultMatchBy
	}
	pairOf, taken := pairElements(want, got, matchBy)

	var items []ItemDiff
	elem := path + "[]"
	for gi, g := range got {
		if !taken[gi] {
			items = append(items, ItemDiff{Before: gi, After: -1, Diff: c.redact(elem, &Diff{Kind: DiffRemoved, Before: g})})
		}
	}
	for wi, w := range want {
		gi := pairOf[wi]
		if gi < 0 {
			items = append(items, ItemDiff{Before: -1, After: wi, Diff: c.redact(elem, &Diff{Kind: DiffAdded, After: w})})
			continue
		}
		d := c.diffAny(elem, w, got[gi], true)
		if d != nil || gi != wi {
			items = append(items, ItemDiff{Before: gi, After: wi, Diff: d})
		}
	}
	if len(items) == 0 {
		return nil
	}
	slices.SortStableFunc(items, func(a, b ItemDiff) int { return cmp.Compare(a.pos(), b.pos()) })
	return &Diff{Kind: DiffList, Items: items}
}

// pairElements pairs each desired element with a server element: first by
// each identity key in matchBy, then by position. pairOf maps a desired index
// to its server index, or -1; taken marks the server elements that were paired.
func pairElements(want, got []any, matchBy []string) (pairOf []int, taken []bool) {
	pairOf = make([]int, len(want))
	taken = make([]bool, len(got))
	for i := range pairOf {
		pairOf[i] = -1
	}
	for _, key := range append(slices.Clone(matchBy), "") {
		for wi, w := range want {
			if pairOf[wi] >= 0 {
				continue
			}
			wid, ok := identity(w, key, wi)
			if !ok {
				continue
			}
			for gi, g := range got {
				if taken[gi] {
					continue
				}
				if gid, ok := identity(g, key, gi); ok && gid == wid {
					pairOf[wi], taken[gi] = gi, true
					break
				}
			}
		}
	}
	return pairOf, taken
}

func (it ItemDiff) pos() int {
	if it.After >= 0 {
		return it.After
	}
	return it.Before
}

// identity names a list element for pairing. An empty key means position.
func identity(v any, key string, index int) (string, bool) {
	if key == "" {
		return fmt.Sprint(index), true
	}
	m, ok := v.(map[string]any)
	if !ok {
		return "", false
	}
	switch id := m[key].(type) {
	case string:
		return id, id != ""
	case nil, map[string]any, []any:
		return "", false
	default:
		return fmt.Sprint(id), true
	}
}

// touches reports whether anything at or beneath a dotted path differs.
func (d *Diff) touches(path string) bool {
	for _, seg := range splitPath(path) {
		if d == nil {
			return false
		}
		if d.Kind != DiffObject {
			// The whole subtree was replaced, added or removed.
			return true
		}
		d = d.Fields[seg]
	}
	return d != nil
}

// clears lists the top-level fields the diff removes outright, which apply
// turns into explicit nulls.
func (d *Diff) clears() []string {
	if d == nil {
		return nil
	}
	var out []string
	for k, f := range d.Fields {
		if f.Kind == DiffRemoved {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

// removedMetadataKeys lists, sorted, the keys of got that want no longer
// declares. Apply sends each one as null.
func removedMetadataKeys(want, got map[string]any) []string {
	var out []string
	for _, k := range slices.Sorted(maps.Keys(got)) {
		if _, kept := want[k]; !kept {
			out = append(out, k)
		}
	}
	return out
}

// checkImmutable refuses an update that would need a field the API will not
// change. Silently recreating the resource would break every reference to it,
// so this is the user's decision to make.
func (c *Change) checkImmutable() error {
	var offenders []string
	for _, path := range c.spec.immutable {
		// A field the server does not report cannot be shown to have changed;
		// that judgement is left to the API.
		if _, known := getPath(c.Remote, path); known && c.Diff.touches(path) {
			offenders = append(offenders, path)
		}
	}
	if len(offenders) == 0 {
		return nil
	}
	return fmt.Errorf("%s cannot be changed after the resource is created; delete %s and apply again to replace it",
		strings.Join(offenders, " and "), c.Key)
}

// redact withholds a leaf's values when a secret lies at or beneath its path.
// Interior nodes need no such care: they descend, and the secret leaf redacts
// itself. A leaf prints its whole value, so a secret anywhere inside leaks.
func (c *Change) redact(path string, d *Diff) *Diff {
	if c.IsSensitive(path) {
		return &Diff{Kind: DiffSensitive}
	}
	return d
}

// declaredSensitive reports whether path itself is listed in Sensitive;
// IsSensitive also counts paths beneath it.
func (c *Change) declaredSensitive(path string) bool {
	flat := flattenPath(path)
	for s := range c.Sensitive {
		if flattenPath(s) == flat {
			return true
		}
	}
	return false
}

// IsSensitive reports whether printing the whole value at path could disclose
// a secret: one declared at the path, or anywhere beneath it. List markers
// ("[]") are ignored on both sides, so "resources[].token" shields "resources"
// and "resources[]" alike.
func (c *Change) IsSensitive(path string) bool {
	flat := flattenPath(path)
	for s := range c.Sensitive {
		fs := flattenPath(s)
		if fs == flat || strings.HasPrefix(fs, flat+".") {
			return true
		}
	}
	return false
}

func flattenPath(p string) string { return strings.ReplaceAll(p, "[]", "") }

// IsEmptyValue reports whether v holds nothing: nil, the empty string, or an
// empty list or map. Diffing and rendering treat such a value the same as an
// absent one.
func IsEmptyValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	}
	return false
}

// equivalent reports whether the server's value already satisfies what we want.
// It is deliberately asymmetric: the API normalizes input (omitted sub-fields
// gain defaults), so "desired is contained in remote" is the honest
// comparison. A stricter one would report a change on every plan and never
// converge.
func equivalent(want, got any) bool {
	if IsEmptyValue(want) && IsEmptyValue(got) {
		return true
	}
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return false
		}
		for k, wv := range w {
			gv, present := g[k]
			if !present {
				if IsEmptyValue(wv) {
					continue
				}
				return false
			}
			if !equivalent(wv, gv) {
				return false
			}
		}
		return true
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for i := range w {
			if !equivalent(w[i], g[i]) {
				return false
			}
		}
		return true
	case string:
		g, ok := got.(string)
		return ok && g == w
	default:
		wj, err1 := canonicalJSON(want)
		gj, err2 := canonicalJSON(got)
		if err1 != nil || err2 != nil {
			return false
		}
		return string(wj) == string(gj)
	}
}
