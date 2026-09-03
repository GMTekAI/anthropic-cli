package conformance

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergePatchFollowsRFC7396(t *testing.T) {
	got := mergePatch(
		map[string]any{"a": 1, "b": map[string]any{"x": 1, "y": 2}, "c": []any{1, 2}},
		map[string]any{"a": nil, "b": map[string]any{"y": nil, "z": 3}, "c": []any{9}},
	)
	assert.Equal(t, map[string]any{"b": map[string]any{"x": 1, "z": 3}, "c": []any{9}}, got)
}

func TestStepsAreDeltasAndRemovalIsNull(t *testing.T) {
	c := load(t, `
schema: 1
name: deltas
resources:
  a: {kind: agent, body: {name: "a-{run}", system: one, tools: [{type: x}]}}
  s: {kind: skill, files: {SKILL.md: "name: s-{run}\n", notes.md: hi}}
steps:
  - expect: {plan: {a: create, s: create}}
  - change:
      a: {body: {system: two, tools: null}}
      s: {files: {notes.md: null}}
      b: {kind: agent, body: {name: "b-{run}", lead: {$ref: a}}}
  - change: {a: null, b: {body: {lead: null}}}
`)
	s1, err := nextDesired(c.Resources, c.Steps[1].Change)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"name": "a-RUN", "system": "two"}, s1["a"].Body)
	assert.Equal(t, map[string]string{"SKILL.md": "name: s-RUN\n"}, s1["s"].Files)
	assert.Equal(t, Ref{Name: "a"}, s1["b"].Body["lead"])

	s2, err := nextDesired(s1, c.Steps[2].Change)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"b", "s"}, s2.Names())
}

func TestValidationCatchesWhatWouldFailThreeCallsIn(t *testing.T) {
	for src, want := range map[string]string{
		`resources: {a: {kind: agent, body: {name: fixed}}}
steps: [{expect: {}}]`: "must contain {run}",
		`resources: {a: {kind: agent, body: {name: "a-{run}", peer: {$ref: nobody}}}}
steps: [{expect: {}}]`: `\$ref "nobody"`,
		`resources: {a: {kind: agent, body: {name: "a-{run}"}}}
steps: [{change: {a: {body: {x: 1}}}}]`: "first step",
		`resources: {a: {kind: agent, body: {name: "a-{run}"}}}
steps: [{expect: {}}, {change: {b: {body: {name: "b-{run}"}}}}]`: "needs a kind",
		`resources: {a: {kind: agent, body: {name: "a-{run}"}}}
steps: [{flags: [yolo]}]`: "unknown flag",
		`resources: {a: {kind: agent, body: {name: "a-{run}"}}}
steps: [{expect: {plan: {a: mutate}}}]`: "unknown action",
	} {
		_, err := parseAndValidate(t, "schema: 1\nname: t\n"+src)
		require.Error(t, err, src)
		assert.Regexp(t, want, err.Error(), src)
	}
}

func TestMatchersAndPaths(t *testing.T) {
	obj := map[string]any{
		"name": "x", "n": 2.0, "tags": []any{"a", "b"},
		"agent": map[string]any{"id": "agent_1", "version": 3.0},
		"list":  []any{map[string]any{"id": "m_1"}},
	}
	r := fakeResolver{ids: map[string]string{"lead": "agent_1", "notes": "m_1"}, versions: map[string]string{"lead": "3"}}
	for _, tc := range []struct {
		path string
		want any
	}{
		{"name", "x"},
		{"n", 2},
		{"tags", []any{"a"}},
		{"tags[1]", "b"},
		{"agent.id", map[string]any{"$id": "lead"}},
		{"agent.version", map[string]any{"$version": "lead"}},
		{"list[0].memory", map[string]any{"$unset": true}},
		{"list[0].id", map[string]any{"$id": "notes"}},
		{"agent", map[string]any{"$set": true}},
		{"name", map[string]any{"$match": "^x$"}},
	} {
		got, ok := lookup(obj, tc.path)
		assert.NoError(t, match(context.Background(), got, ok, tc.want, r), tc.path)
	}
	for path, want := range map[string]any{
		"name":     "y",
		"tags":     []any{"b"},
		"agent.id": map[string]any{"$id": "notes"},
		"missing":  1,
		"agent":    map[string]any{"$unset": true},
	} {
		got, ok := lookup(obj, path)
		assert.Error(t, match(context.Background(), got, ok, want, r), path)
	}
}

type fakeResolver struct{ ids, versions map[string]string }

func (f fakeResolver) id(n string) (string, bool) { id, ok := f.ids[n]; return id, ok }
func (f fakeResolver) version(_ context.Context, n string) (string, error) {
	return f.versions[n], nil
}

// The runner against a toy tool and a toy server: enough to pin down the
// sequencing — render before plan, re-plan after apply, remote actions hit
// the server not the tool, teardown verified — without a network.
func TestRunnerDrivesAToolThroughACase(t *testing.T) {
	c := load(t, `
schema: 1
name: toy
resources:
  a: {kind: widget, body: {name: "a-{run}", colour: red}}
  b: {kind: widget, body: {name: "b-{run}", peer: {$ref: a}}}
steps:
  - expect:
      plan: {a: create, b: create}
      remote: {b: {peer: {$id: a}, name: "b-{run}"}}
  - change: {a: {body: {colour: blue}}}
    expect:
      plan: {a: {update: [colour]}}
      remote: {a: {colour: blue}}
  - remote: {a: {patch: {colour: green}}}
    expect: {error: drift}
  - flags: [force]
    expect: {plan: {a: update}, remote: {a: {colour: blue}}}
  - remote: {b: delete}
    change: {b: null}
    flags: [prune]
    expect: {plan: {b: destroy}}
`)
	srv := &toyServer{objects: map[string]map[string]any{}}
	tool := &toyTool{srv: srv}
	Run(t, []*Case{c}, tool, srv, Options{})

	assert.Empty(t, srv.live(), "teardown removes everything")
	assert.Equal(t, []string{
		"render", "plan", "apply", "plan",
		"render", "plan", "apply", "plan",
		"server:patch", "render", "plan",
		"render", "plan", "apply", "plan",
		"server:destroy", "render", "plan", "apply", "plan",
		"destroy",
	}, tool.log)
}

func TestAnUnlistedResourceMustPlanAsNoop(t *testing.T) {
	diff := comparePlan(
		Plan{"a": {Action: ActionCreate}, "b": {Action: ActionUpdate, Fields: []string{"peer"}}},
		Plan{"a": {Action: ActionCreate}},
		Desired{"a": {}, "b": {}})
	assert.Contains(t, diff, "b ")
	assert.Contains(t, diff, "want noop")
	assert.Contains(t, diff, "got update[peer]")
	assert.NotContains(t, diff, "a ")
}

// --- helpers ---------------------------------------------------------------

func load(t *testing.T, src string) *Case {
	t.Helper()
	c, err := parseAndValidate(t, src)
	require.NoError(t, err)
	return c
}

func parseAndValidate(t *testing.T, src string) (*Case, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "case.yaml")
	require.NoError(t, os.WriteFile(path, []byte(src), 0o644))
	return LoadFile(path, "RUN")
}

// toyServer is an in-memory API with drift: patch changes an object behind
// the tool's back.
type toyServer struct {
	objects map[string]map[string]any
	seq     int
	tool    *toyTool
}

func (s *toyServer) live() []string {
	var ids []string
	for id, o := range s.objects {
		if o["archived_at"] == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

func (s *toyServer) Get(_ context.Context, _, id string) (map[string]any, error) {
	o, ok := s.objects[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return o, nil
}

func (s *toyServer) Update(_ context.Context, _, id string, body map[string]any, _ map[string]string) error {
	s.tool.log = append(s.tool.log, "server:patch")
	for k, v := range body {
		s.objects[id][k] = v
	}
	s.objects[id]["rev"] = s.objects[id]["rev"].(int) + 1
	return nil
}

func (s *toyServer) Destroy(_ context.Context, _, id string) error {
	s.tool.log = append(s.tool.log, "server:destroy")
	delete(s.objects, id)
	return nil
}

// toyTool is a minimal reconciler: it remembers what it wrote (state),
// diffs desired against the server, refuses drift unless forced.
type toyTool struct {
	srv     *toyServer
	desired Desired
	state   map[string]toyEntry // logical name → id, rev at last write
	log     []string
}

type toyEntry struct {
	id  string
	rev int
}

func (tt *toyTool) Name() string           { return "toy" }
func (tt *toyTool) Supports(k string) bool { return k == "widget" }
func (tt *toyTool) Start(_ *testing.T) Session {
	tt.state = map[string]toyEntry{}
	tt.srv.tool = tt
	return tt
}

func (tt *toyTool) Render(d Desired) error {
	tt.log = append(tt.log, "render")
	tt.desired = d
	return nil
}

func (tt *toyTool) body(name string) map[string]any {
	out := map[string]any{}
	for k, v := range tt.desired[name].Body {
		if r, ok := v.(Ref); ok {
			v = tt.state[r.Name].id
		}
		out[k] = v
	}
	return out
}

func (tt *toyTool) Plan(_ context.Context, f Flags) (Plan, error) {
	tt.log = append(tt.log, "plan")
	return tt.summarize(f)
}

// summarize is Plan without the log entry, so Apply's own planning is not
// recorded as a plan the runner asked for.
func (tt *toyTool) summarize(f Flags) (Plan, error) {
	out := Plan{}
	for name := range tt.desired {
		e, known := tt.state[name]
		obj, exists := tt.srv.objects[e.id]
		switch {
		case !known:
			out[name] = Planned{Action: ActionCreate}
		case !exists:
			if !f.Force {
				return nil, fmt.Errorf("%s: gone; drift", name)
			}
			out[name] = Planned{Action: ActionReplace}
		default:
			if obj["rev"].(int) != e.rev && !f.Force {
				return nil, fmt.Errorf("%s: drift", name)
			}
			var fields []string
			for k, v := range tt.body(name) {
				if fmt.Sprint(obj[k]) != fmt.Sprint(v) {
					fields = append(fields, k)
				}
			}
			sort.Strings(fields)
			if len(fields) > 0 {
				out[name] = Planned{Action: ActionUpdate, Fields: fields}
			} else {
				out[name] = Planned{Action: ActionNoop}
			}
		}
	}
	for name := range tt.state {
		if _, ok := tt.desired[name]; !ok && f.Prune {
			out[name] = Planned{Action: ActionDestroy}
		}
	}
	return out, nil
}

func (tt *toyTool) Apply(_ context.Context, f Flags) error {
	tt.log = append(tt.log, "apply")
	plan, err := tt.summarize(f)
	if err != nil {
		return err
	}
	names := sortedKeys(plan)
	// crude dependency order: referenced things first
	sort.SliceStable(names, func(i, j int) bool { return len(tt.desired[names[i]].Body) < len(tt.desired[names[j]].Body) })
	for _, name := range names {
		switch plan[name].Action {
		case ActionCreate, ActionReplace:
			tt.srv.seq++
			id := fmt.Sprintf("w_%d", tt.srv.seq)
			obj := tt.body(name)
			obj["id"], obj["rev"] = id, 1
			tt.srv.objects[id] = obj
			tt.state[name] = toyEntry{id: id, rev: 1}
		case ActionUpdate:
			e := tt.state[name]
			obj := tt.srv.objects[e.id]
			for k := range obj {
				if k != "id" && k != "rev" {
					delete(obj, k)
				}
			}
			for k, v := range tt.body(name) {
				obj[k] = v
			}
			obj["rev"] = obj["rev"].(int) + 1
			tt.state[name] = toyEntry{id: e.id, rev: obj["rev"].(int)}
		case ActionDestroy:
			delete(tt.srv.objects, tt.state[name].id)
			delete(tt.state, name)
		}
	}
	return nil
}

func (tt *toyTool) IDs() map[string]string {
	out := map[string]string{}
	for n, e := range tt.state {
		out[n] = e.id
	}
	return out
}

func (tt *toyTool) Destroy(ctx context.Context) error {
	tt.log = append(tt.log, "destroy")
	for n, e := range tt.state {
		delete(tt.srv.objects, e.id)
		delete(tt.state, n)
	}
	return nil
}
