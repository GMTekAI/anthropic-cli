package conformance

import (
	"fmt"
	"maps"
)

// nextDesired produces the next desired state from the previous one and a
// step's `change` block. prev is left unmodified.
func nextDesired(prev Desired, changes map[string]*Patch) (Desired, error) {
	next := Desired{}
	for n, r := range prev {
		next[n] = Resource{Kind: r.Kind, Body: cloneMap(r.Body), Files: cloneFiles(r.Files)}
	}
	for name, p := range changes {
		if p == nil {
			if _, ok := next[name]; !ok {
				return nil, fmt.Errorf("change.%s: null removes a resource, but there is none by that name", name)
			}
			delete(next, name)
			continue
		}
		r, exists := next[name]
		if !exists {
			if p.Kind == "" {
				return nil, fmt.Errorf("change.%s: a new resource needs a kind", name)
			}
			r = Resource{Kind: p.Kind, Body: map[string]any{}, Files: map[string]string{}}
		} else if p.Kind != "" && p.Kind != r.Kind {
			return nil, fmt.Errorf("change.%s: kind cannot change (%s → %s); remove it and add another", name, r.Kind, p.Kind)
		}
		if p.Body != nil {
			r.Body = mergePatch(r.Body, p.Body).(map[string]any)
		}
		for f, c := range p.Files {
			if c == nil {
				delete(r.Files, f)
				continue
			}
			s, ok := c.(string)
			if !ok {
				return nil, fmt.Errorf("change.%s.files.%s: want a string or null", name, f)
			}
			r.Files[f] = s
		}
		next[name] = r
	}
	return next, nil
}

// mergePatch applies RFC 7396: objects merge key by key, null deletes, and
// any other value — lists included — replaces outright. target is left
// unmodified.
func mergePatch(target, patch any) any {
	pm, ok := patch.(map[string]any)
	if !ok {
		return patch
	}
	tm, ok := target.(map[string]any)
	if !ok {
		tm = map[string]any{}
	}
	out := cloneMap(tm)
	for k, v := range pm {
		if v == nil {
			delete(out, k)
			continue
		}
		out[k] = mergePatch(out[k], v)
	}
	return out
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = cloneAny(v)
	}
	return out
}

func cloneAny(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return cloneMap(t)
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = cloneAny(e)
		}
		return out
	}
	return v
}

func cloneFiles(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	maps.Copy(out, m)
	return out
}
