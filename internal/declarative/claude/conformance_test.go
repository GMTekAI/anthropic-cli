package claude_test

// The ant adapter for the shared conformance cases, and the test that runs
// them end to end. Opt-in, because it creates and destroys real resources:
//
//	ANT_CONFORMANCE=1 go test ./internal/declarative/claude/ -run TestConformance -v
//
// Credentials come from the SDK's default chain (ANTHROPIC_API_KEY,
// ANTHROPIC_AUTH_TOKEN, ANTHROPIC_PROFILE, federation), so point them at a
// scratch organization. -run TestConformance/<case name> runs one case.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-cli/internal/declarative/claude"
	"github.com/anthropics/anthropic-cli/internal/declarative/conformance"
	"github.com/anthropics/anthropic-cli/internal/declarative/core"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/stretchr/testify/require"
)

func TestConformance(t *testing.T) {
	if os.Getenv("ANT_CONFORMANCE") == "" {
		t.Skip("end-to-end: set ANT_CONFORMANCE=1 and credentials for a scratch organization")
	}
	runID := conformance.NewRunID()
	cases, err := conformance.Load("../conformance/cases", runID)
	require.NoError(t, err)
	t.Logf("run id %s — leaked resources, if any, are named cf-%s-*", runID, runID)

	client := claude.NewClient(anthropic.NewClient())
	conformance.Run(t, cases, antAdapter{client}, antRemote{client}, conformance.Options{Parallel: true})
}

// Cases must at least load, whether or not anyone is running them today.
func TestConformanceCasesAreWellFormed(t *testing.T) {
	cases, err := conformance.Load("../conformance/cases", "wellformed")
	require.NoError(t, err)
	require.NotEmpty(t, cases)
	for _, c := range cases {
		for _, k := range c.Kinds() {
			require.True(t, claude.Registry().Valid(core.Kind(k)) || !antAdapter{}.Supports(k), "%s: kind %q", c.Path, k)
		}
	}
}

// antAdapter renders desired state the way a person writes it for `ant
// apply` — one file per resource under a directory named for its kind, skills
// as directories, references as relative paths — and drives the planner and
// applier directly rather than through the command, so a failure names the
// resource and not an exit code.
type antAdapter struct{ client core.Client }

func (antAdapter) Name() string { return "ant" }

func (antAdapter) Supports(kind string) bool { return claude.Registry().Valid(core.Kind(kind)) }

func (a antAdapter) Start(t *testing.T) conformance.Session {
	root := t.TempDir()
	lock, err := core.LoadLockfile(claude.Registry(), filepath.Join(root, claude.Registry().LockfileName()))
	require.NoError(t, err)
	return &antSession{t: t, root: root, client: a.client, lock: lock, names: map[string]string{}}
}

type antSession struct {
	t      *testing.T
	root   string
	client core.Client
	lock   *core.Lockfile
	// names maps a lockfile key back to the case's logical name.
	names map[string]string
}

// dirs is where each kind's files go; it doubles as the directory-name
// convention that tells ant what kind a file is.
var dirs = map[string]string{
	"skill":        "skills",
	"environment":  "environments",
	"memory_store": "memory_stores",
	"agent":        "agents",
	"deployment":   "deployments",
}

func (s *antSession) pathFor(name string, r conformance.Resource) string {
	if r.Kind == "skill" {
		return filepath.Join(dirs[r.Kind], name)
	}
	return filepath.Join(dirs[r.Kind], name+".json")
}

func (s *antSession) Render(desired conformance.Desired) error {
	for _, d := range dirs {
		if err := os.RemoveAll(filepath.Join(s.root, d)); err != nil {
			return err
		}
	}
	kinds := map[string]string{}
	for name, r := range desired {
		kinds[name] = r.Kind
	}
	for _, name := range desired.Names() {
		r := desired[name]
		rel := s.pathFor(name, r)
		s.names["./"+filepath.ToSlash(rel)] = name
		if r.Kind == "skill" {
			for f, content := range r.Files {
				if err := writeFile(filepath.Join(s.root, rel, f), content); err != nil {
					return err
				}
			}
			continue
		}
		body := s.refsToPaths(r.Body, rel, desired).(map[string]any)
		data, err := json.MarshalIndent(body, "", "  ")
		if err != nil {
			return err
		}
		if err := writeFile(filepath.Join(s.root, rel), string(data)+"\n"); err != nil {
			return err
		}
	}
	return nil
}

// refsToPaths replaces each Ref with the relative path from the referring
// file to the referenced one, which is how ant spells a reference anywhere.
func (s *antSession) refsToPaths(v any, from string, desired conformance.Desired) any {
	switch t := v.(type) {
	case conformance.Ref:
		target := s.pathFor(t.Name, desired[t.Name])
		rel, _ := filepath.Rel(filepath.Dir(from), target)
		return "./" + filepath.ToSlash(rel)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = s.refsToPaths(val, from, desired)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = s.refsToPaths(val, from, desired)
		}
		return out
	}
	return v
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func (s *antSession) plan(ctx context.Context, flags conformance.Flags) (*core.Plan, error) {
	loader := core.NewLoader(claude.Registry(), s.root, nil)
	if err := loader.Add(ctx, []string{s.root}); err != nil {
		return nil, err
	}
	if err := loader.AddKeys(ctx, s.lock.Keys()); err != nil {
		return nil, err
	}
	planner := &core.Planner{Registry: claude.Registry(), Client: s.client, Lock: s.lock, Force: flags.Force, Prune: flags.Prune}
	plan, err := planner.Plan(ctx, loader)
	if err != nil {
		return nil, err
	}
	// A plan ant would refuse to apply is, to the case, an error.
	var msgs []string
	for _, c := range plan.Blocked() {
		msgs = append(msgs, fmt.Sprintf("%s: %v %s", s.nameFor(c.Key), c.Blocked, strings.Join(c.Reasons, "; ")))
	}
	if len(msgs) > 0 {
		return nil, errors.New(strings.Join(msgs, "\n"))
	}
	return plan, nil
}

func (s *antSession) Plan(ctx context.Context, flags conformance.Flags) (conformance.Plan, error) {
	plan, err := s.plan(ctx, flags)
	if err != nil {
		return nil, err
	}
	out := conformance.Plan{}
	for _, c := range plan.Changes {
		p := conformance.Planned{}
		switch c.Action {
		case core.ActionCreate:
			p.Action = conformance.ActionCreate
			if c.Replaces != "" {
				p.Action = conformance.ActionReplace
			}
		case core.ActionUpdate:
			p.Action = conformance.ActionUpdate
			if c.Diff != nil {
				for f := range c.Diff.Fields {
					p.Fields = append(p.Fields, f)
				}
				sort.Strings(p.Fields)
			}
		case core.ActionDestroy:
			p.Action = conformance.ActionDestroy
		default:
			p.Action = conformance.ActionNoop
		}
		out[s.nameFor(c.Key)] = p
	}
	return out, nil
}

func (s *antSession) Apply(ctx context.Context, flags conformance.Flags) error {
	plan, err := s.plan(ctx, flags)
	if err != nil {
		return err
	}
	_, err = (&core.Applier{Client: s.client, Lock: s.lock}).Apply(ctx, plan)
	if err != nil {
		return err
	}
	// Reload from disk, so the cases also prove the lockfile round-trips.
	s.lock, err = core.LoadLockfile(claude.Registry(), s.lock.Path)
	return err
}

func (s *antSession) IDs() map[string]string {
	out := map[string]string{}
	for key, e := range s.lock.Resources {
		out[s.nameFor(key)] = e.ID
	}
	return out
}

func (s *antSession) Destroy(ctx context.Context) error {
	if err := s.Render(conformance.Desired{}); err != nil {
		return err
	}
	return s.Apply(ctx, conformance.Flags{Force: true, Prune: true})
}

func (s *antSession) nameFor(key string) string {
	if n, ok := s.names[key]; ok {
		return n
	}
	return key
}

// antRemote is the runner's line to the API. It goes through the same client
// the reconciler uses because that client already speaks every kind; the
// runner only ever asks it for reads, patches and removals by id.
type antRemote struct{ client core.Client }

func (r antRemote) Get(ctx context.Context, kind, id string) (map[string]any, error) {
	obj, err := r.client.Get(ctx, core.Kind(kind), id)
	if errors.Is(err, core.ErrNotFound) {
		return nil, fmt.Errorf("%w: %s %s", conformance.ErrNotFound, kind, id)
	}
	return obj, err
}

func (r antRemote) Update(ctx context.Context, kind, id string, body map[string]any, files map[string]string) error {
	req := core.Request{Body: body}
	if len(files) > 0 {
		dir, err := os.MkdirTemp("", "cf-remote-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		for f, content := range files {
			if err := writeFile(filepath.Join(dir, f), content); err != nil {
				return err
			}
		}
		_, fields, payload, err := claude.Registry().Schema().LoadDir(dir)
		if err != nil {
			return err
		}
		req = core.Request{Body: fields, Payload: payload}
	} else if spec, _ := claude.Registry().Spec(core.Kind(kind)); spec.UpdateNeedsVersion {
		obj, err := r.client.Get(ctx, core.Kind(kind), id)
		if err != nil {
			return err
		}
		req.Body = mergeVersion(body, obj)
	}
	_, err := r.client.Update(ctx, core.Kind(kind), id, req)
	return err
}

// mergeVersion adds the concurrency token an agent update must carry.
func mergeVersion(body, remote map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range body {
		out[k] = v
	}
	out["version"] = remote["version"]
	return out
}

func (r antRemote) Destroy(ctx context.Context, kind, id string) error {
	return r.client.Destroy(ctx, core.Kind(kind), id)
}
