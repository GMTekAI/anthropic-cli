package core

import "strings"

// Dotted-path helpers for walking a decoded JSON body. Paths are simple:
// dot-separated map keys, plus a "[]" segment meaning "every element of this
// array". That covers everything the kind tables need and stays predictable in
// error messages, which full JSONPath would not.

func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	return strings.Split(path, ".")
}

func getPath(body map[string]any, path string) (any, bool) {
	var cur any = body
	for _, seg := range splitPath(path) {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[seg]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// setPath writes a value at a dotted path, creating intermediate maps. It is a
// no-op if an intermediate segment exists but is not a map.
func setPath(body map[string]any, path string, value any) {
	segs := splitPath(path)
	if len(segs) == 0 {
		return
	}
	cur := body
	for _, seg := range segs[:len(segs)-1] {
		next, ok := cur[seg].(map[string]any)
		if !ok {
			if _, exists := cur[seg]; exists {
				return
			}
			next = map[string]any{}
			cur[seg] = next
		}
		cur = next
	}
	cur[segs[len(segs)-1]] = value
}

func deletePath(body map[string]any, path string) {
	segs := splitPath(path)
	if len(segs) == 0 {
		return
	}
	cur := body
	for _, seg := range segs[:len(segs)-1] {
		next, ok := cur[seg].(map[string]any)
		if !ok {
			return
		}
		cur = next
	}
	delete(cur, segs[len(segs)-1])
}

// deleteWildcardPath removes a field from every element of an array, given a
// path of the form "resources[].authorization_token".
func deleteWildcardPath(body map[string]any, path string) {
	prefix, rest, found := strings.Cut(path, "[].")
	if !found {
		deletePath(body, path)
		return
	}
	raw, ok := getPath(body, prefix)
	if !ok {
		return
	}
	items, ok := raw.([]any)
	if !ok {
		return
	}
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			deleteWildcardPath(m, rest)
		}
	}
}

// deepCopy clones a decoded JSON value so callers can mutate a body without
// disturbing the parsed source it came from.
func deepCopy(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = deepCopy(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = deepCopy(val)
		}
		return out
	default:
		return v
	}
}

// deepCopyMap is deepCopy for a body. The result is never nil.
func deepCopyMap(m map[string]any) map[string]any {
	return deepCopy(m).(map[string]any)
}
