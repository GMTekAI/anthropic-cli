package conformance

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

// NewRunID returns the token substituted for {run}.
func NewRunID() string {
	var b [4]byte
	rand.Read(b[:]) // never returns an error; see crypto/rand.Read
	return hex.EncodeToString(b[:])
}

// Load reads and validates every .yaml and .yml case under dir, recursively,
// substituting runID for {run}. Cases come back sorted by path; the first file
// that fails to load or validate stops the walk and is returned as the error.
func Load(dir, runID string) ([]*Case, error) {
	var cases []*Case
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		switch filepath.Ext(path) {
		case ".yaml", ".yml":
		default:
			return nil
		}
		c, err := LoadFile(path, runID)
		if err != nil {
			return err
		}
		cases = append(cases, c)
		return nil
	})
	slices.SortFunc(cases, func(a, b *Case) int { return strings.Compare(a.Path, b.Path) })
	return cases, err
}

// LoadFile reads and validates one case, substituting runID for {run}.
func LoadFile(path, runID string) (*Case, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c, err := parse(strings.ReplaceAll(string(raw), "{run}", runID))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	c.Path = path
	if err := c.validate(runID); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

// parse decodes a case through generic maps rather than tagged structs,
// because half the format is "a string or an object here", which reads better
// in a case file than it decodes into Go.
func parse(src string) (*Case, error) {
	var raw any
	if err := yaml.Unmarshal([]byte(src), &raw); err != nil {
		return nil, err
	}
	doc, ok := normalize(raw).(map[string]any)
	if !ok {
		return nil, fmt.Errorf("want a mapping at the top level")
	}
	if i, ok := asInt(doc["schema"]); !ok || i != SchemaVersion {
		return nil, fmt.Errorf("schema: want %d, got %v", SchemaVersion, doc["schema"])
	}
	c := &Case{Skip: map[string]string{}}
	for k, v := range doc {
		switch k {
		case "schema":
		case "name":
			c.Name, _ = v.(string)
		case "skip":
			m, ok := asMap(v)
			if !ok {
				return nil, fmt.Errorf("skip: want {tool: reason}")
			}
			for tool, why := range m {
				c.Skip[tool], _ = why.(string)
			}
		case "only":
			for _, t := range asList(v) {
				s, _ := t.(string)
				c.Only = append(c.Only, s)
			}
		case "resources":
			m, ok := asMap(v)
			if !ok {
				return nil, fmt.Errorf("resources: want a mapping of name to resource")
			}
			c.Resources = Desired{}
			for name, rv := range m {
				r, err := parseResource(name, rv)
				if err != nil {
					return nil, err
				}
				c.Resources[name] = r
			}
		case "steps":
			for i, sv := range asList(v) {
				s, err := parseStep(sv)
				if err != nil {
					return nil, fmt.Errorf("steps[%d]: %w", i, err)
				}
				c.Steps = append(c.Steps, s)
			}
		default:
			return nil, fmt.Errorf("unknown top-level key %q", k)
		}
	}
	if c.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	return c, nil
}

// parseResource reads one entry under `resources`. Unlike a patch under a
// step's `change`, it must name its kind: there is no earlier state to take
// the kind from.
func parseResource(name string, v any) (Resource, error) {
	p, err := parsePatch(v)
	if err != nil {
		return Resource{}, fmt.Errorf("resources.%s: %w", name, err)
	}
	if p == nil || p.Kind == "" {
		return Resource{}, fmt.Errorf("resources.%s: needs a kind", name)
	}
	r := Resource{Kind: p.Kind, Body: p.Body, Files: map[string]string{}}
	if r.Body == nil {
		r.Body = map[string]any{}
	}
	for f, content := range p.Files {
		s, ok := content.(string)
		if !ok {
			return Resource{}, fmt.Errorf("resources.%s.files.%s: want a string", name, f)
		}
		r.Files[f] = s
	}
	return r, nil
}

func parsePatch(v any) (*Patch, error) {
	if v == nil {
		return nil, nil
	}
	m, ok := asMap(v)
	if !ok {
		return nil, fmt.Errorf("want {kind, body, files}")
	}
	p := &Patch{}
	for k, val := range m {
		switch k {
		case "kind":
			p.Kind, _ = val.(string)
		case "body":
			body, ok := asMap(val)
			if !ok {
				return nil, fmt.Errorf("body: want a mapping")
			}
			p.Body = decodeRefs(body).(map[string]any)
		case "files":
			files, ok := asMap(val)
			if !ok {
				return nil, fmt.Errorf("files: want a mapping of path to content")
			}
			p.Files = files
		default:
			return nil, fmt.Errorf("unknown key %q", k)
		}
	}
	return p, nil
}

// decodeRefs rewrites every {$ref: name} in a body into a Ref.
func decodeRefs(v any) any {
	switch t := v.(type) {
	case map[string]any:
		if name, ok := t["$ref"].(string); ok && len(t) == 1 {
			return Ref{Name: name}
		}
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = decodeRefs(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = decodeRefs(val)
		}
		return out
	}
	return v
}

func parseStep(v any) (Step, error) {
	m, ok := asMap(v)
	if !ok {
		return Step{}, fmt.Errorf("want a mapping")
	}
	var s Step
	for k, val := range m {
		switch k {
		case "change":
			cm, ok := asMap(val)
			if !ok {
				return Step{}, fmt.Errorf("change: want {name: patch}")
			}
			s.Change = map[string]*Patch{}
			for name, pv := range cm {
				p, err := parsePatch(pv)
				if err != nil {
					return Step{}, fmt.Errorf("change.%s: %w", name, err)
				}
				s.Change[name] = p
			}
		case "remote":
			rm, ok := asMap(val)
			if !ok {
				return Step{}, fmt.Errorf("remote: want {name: action}")
			}
			s.Remote = map[string]RemoteAction{}
			for name, av := range rm {
				a, err := parseRemoteAction(av)
				if err != nil {
					return Step{}, fmt.Errorf("remote.%s: %w", name, err)
				}
				s.Remote[name] = a
			}
		case "flags":
			for _, f := range asList(val) {
				switch f {
				case "force":
					s.Flags.Force = true
				case "prune":
					s.Flags.Prune = true
				default:
					return Step{}, fmt.Errorf("flags: unknown flag %v (known: force, prune)", f)
				}
			}
		case "expect":
			e, err := parseExpect(val)
			if err != nil {
				return Step{}, fmt.Errorf("expect.%w", err)
			}
			s.Expect = e
		default:
			return Step{}, fmt.Errorf("unknown key %q", k)
		}
	}
	return s, nil
}

func parseRemoteAction(v any) (RemoteAction, error) {
	if s, ok := v.(string); ok {
		switch s {
		case "archive", "delete":
			return RemoteAction{Verb: s}, nil
		}
		return RemoteAction{}, fmt.Errorf("unknown action %q (known: archive, delete, {patch: ...})", s)
	}
	m, ok := asMap(v)
	if !ok {
		return RemoteAction{}, fmt.Errorf("want archive, delete or {patch: ..., files: ...}")
	}
	a := RemoteAction{Verb: "patch", Files: map[string]string{}}
	for k, val := range m {
		switch k {
		case "patch":
			a.Body, ok = asMap(val)
			if !ok {
				return RemoteAction{}, fmt.Errorf("patch: want a mapping")
			}
		case "files":
			files, ok := asMap(val)
			if !ok {
				return RemoteAction{}, fmt.Errorf("files: want a mapping")
			}
			for f, c := range files {
				a.Files[f], _ = c.(string)
			}
		default:
			return RemoteAction{}, fmt.Errorf("unknown key %q", k)
		}
	}
	return a, nil
}

func parseExpect(v any) (Expect, error) {
	m, ok := asMap(v)
	if !ok {
		return Expect{}, fmt.Errorf(": want a mapping")
	}
	var e Expect
	for k, val := range m {
		switch k {
		case "plan":
			pm, ok := asMap(val)
			if !ok {
				return Expect{}, fmt.Errorf("plan: want {name: action}")
			}
			e.Plan = Plan{}
			for name, pv := range pm {
				p, err := parsePlanned(pv)
				if err != nil {
					return Expect{}, fmt.Errorf("plan.%s: %w", name, err)
				}
				e.Plan[name] = p
			}
		case "error":
			s, _ := val.(string)
			re, err := regexp.Compile(s)
			if err != nil {
				return Expect{}, fmt.Errorf("error: %w", err)
			}
			e.Error = re
		case "remote":
			rm, ok := asMap(val)
			if !ok {
				return Expect{}, fmt.Errorf("remote: want {name: {path: matcher}}")
			}
			e.Remote = map[string]map[string]any{}
			for name, av := range rm {
				paths, ok := asMap(av)
				if !ok {
					return Expect{}, fmt.Errorf("remote.%s: want {path: matcher}", name)
				}
				e.Remote[name] = paths
			}
		default:
			return Expect{}, fmt.Errorf("%s: unknown key", k)
		}
	}
	return e, nil
}

func parsePlanned(v any) (Planned, error) {
	if s, ok := v.(string); ok {
		switch a := Action(s); a {
		case ActionCreate, ActionNoop, ActionUpdate, ActionReplace, ActionDestroy:
			return Planned{Action: a}, nil
		}
		return Planned{}, fmt.Errorf("unknown action %q", s)
	}
	m, ok := asMap(v)
	if !ok || len(m) != 1 {
		return Planned{}, fmt.Errorf("want an action or {update: [fields]}")
	}
	fields, ok := m["update"]
	if !ok {
		return Planned{}, fmt.Errorf("only update takes a field list")
	}
	p := Planned{Action: ActionUpdate}
	for _, f := range asList(fields) {
		s, _ := f.(string)
		p.Fields = append(p.Fields, s)
	}
	slices.Sort(p.Fields)
	return p, nil
}

// normalize gives YAML's maps string keys throughout, so the rest of the
// package can treat a decoded case like decoded JSON.
func normalize(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = normalize(val)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[fmt.Sprint(k)] = normalize(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = normalize(val)
		}
		return out
	}
	return v
}

func asMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func asList(v any) []any {
	l, _ := v.([]any)
	return l
}

func asInt(v any) (int, bool) {
	switch t := v.(type) {
	case int:
		return t, true
	case int64:
		return int(t), true
	case uint64:
		return int(t), true
	case float64:
		return int(t), t == float64(int(t))
	}
	return 0, false
}
