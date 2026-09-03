package conformance

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// resolver answers the questions a matcher can ask about other resources in
// the run: what id a logical name was given, and what version it is at now.
type resolver interface {
	id(name string) (string, bool)
	version(ctx context.Context, name string) (string, error)
}

// match checks one value read back from the API against an expectation from a
// case file. want is a literal (compared loosely: numbers by value, strings
// exactly, lists and objects recursively as subsets) or a single-key matcher
// object: {$id: name}, {$version: name}, {$set: true}, {$unset: true},
// {$match: regex}.
// A matcher must be the whole expectation for its path: nested inside a
// literal, or under any other single key, an object is compared literally.
func match(ctx context.Context, got any, present bool, want any, r resolver) error {
	if op, arg, ok := soleEntry(want); ok {
		switch op {
		case "$set":
			if !present || got == nil {
				return fmt.Errorf("want it set, but it is absent")
			}
			return nil
		case "$unset":
			if present && got != nil {
				return fmt.Errorf("want it absent, got %s", show(got))
			}
			return nil
		case "$match":
			pat, _ := arg.(string)
			re, err := regexp.Compile(pat)
			if err != nil {
				return err
			}
			if s := fmt.Sprint(got); !re.MatchString(s) {
				return fmt.Errorf("want a match for /%s/, got %q", pat, s)
			}
			return nil
		case "$id":
			name, _ := arg.(string)
			id, ok := r.id(name)
			if !ok {
				return fmt.Errorf("$id: %s has no id yet", name)
			}
			if fmt.Sprint(got) != id {
				return fmt.Errorf("want %s's id %s, got %s", name, id, show(got))
			}
			return nil
		case "$version":
			name, _ := arg.(string)
			v, err := r.version(ctx, name)
			if err != nil {
				return fmt.Errorf("$version: %w", err)
			}
			if fmt.Sprint(got) != v {
				return fmt.Errorf("want %s's version %s, got %s", name, v, show(got))
			}
			return nil
		}
		// Any other one-key object is a literal.
	}
	if !present {
		return fmt.Errorf("want %s, but it is absent", show(want))
	}
	return subset(got, want)
}

// soleEntry returns the key and value of a one-key object.
func soleEntry(v any) (key string, val any, ok bool) {
	m, isMap := v.(map[string]any)
	if !isMap || len(m) != 1 {
		return "", nil, false
	}
	for k, x := range m {
		key, val = k, x
	}
	return key, val, true
}

// subset returns nil when want is contained in got: scalars equal, a list's
// leading elements matching want's position by position, and every named
// field present and matching. Extra trailing elements and extra fields are
// ignored. Otherwise the error names the first path where they differ.
func subset(got, want any) error {
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return fmt.Errorf("want an object, got %s", show(got))
		}
		for k, wv := range w {
			gv, ok := g[k]
			if !ok {
				return fmt.Errorf(".%s: absent", k)
			}
			if err := subset(gv, wv); err != nil {
				return fmt.Errorf(".%s%s", k, pathSuffix(err))
			}
		}
		return nil
	case []any:
		g, ok := got.([]any)
		if !ok {
			return fmt.Errorf("want a list, got %s", show(got))
		}
		if len(g) < len(w) {
			return fmt.Errorf("want at least %d elements, got %d", len(w), len(g))
		}
		for i := range w {
			if err := subset(g[i], w[i]); err != nil {
				return fmt.Errorf("[%d]%s", i, pathSuffix(err))
			}
		}
		return nil
	case nil:
		if got != nil {
			return fmt.Errorf("want null, got %s", show(got))
		}
		return nil
	}
	if !scalarEqual(got, want) {
		return fmt.Errorf("want %s, got %s", show(want), show(got))
	}
	return nil
}

// pathSuffix makes nested paths read `.a.b[0]: want …` rather than
// `.a: .b: [0]: want …`.
func pathSuffix(err error) string {
	s := err.Error()
	if strings.HasPrefix(s, ".") || strings.HasPrefix(s, "[") {
		return s
	}
	return ": " + s
}

// scalarEqual compares numbers by value whatever their Go type, since a case
// decodes `2` as an integer and the API's JSON decodes it as a float64.
func scalarEqual(got, want any) bool {
	if gs, ok := got.(string); ok {
		ws, ok := want.(string)
		return ok && gs == ws
	}
	if gb, ok := got.(bool); ok {
		wb, ok := want.(bool)
		return ok && gb == wb
	}
	gn, gok := asFloat(got)
	wn, wok := asFloat(want)
	if gok && wok {
		return gn == wn
	}
	return fmt.Sprint(got) == fmt.Sprint(want)
}

func asFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case uint64:
		return float64(t), true
	}
	return 0, false
}

func show(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

// lookup follows a dotted path — `a.b[0].c` — into a decoded JSON object.
// The second result says whether every segment was present.
func lookup(obj any, path string) (any, bool) {
	cur := obj
	for _, seg := range splitPath(path) {
		switch c := cur.(type) {
		case map[string]any:
			v, ok := c[seg]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(c) {
				return nil, false
			}
			cur = c[i]
		default:
			return nil, false
		}
	}
	return cur, true
}

// splitPath turns `a.b[0].c` into [a b 0 c].
func splitPath(path string) []string {
	path = strings.NewReplacer("[", ".", "]", "").Replace(path)
	var out []string
	for _, s := range strings.Split(path, ".") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
