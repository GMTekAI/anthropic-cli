package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeTree materializes a map of relative path to contents under a temp dir.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	writeTreeAt(t, root, files)
	return root
}

func writeTreeAt(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}
}

// testSchema and testRegistry are a two-kind domain with no relationship to
// any real API: widgets and gadgets, each living in a directory named after
// it. Core is supposed to be domain-agnostic; using the real Claude registry
// in its tests would let a Claude assumption leak back in unnoticed.
type testSchema struct{}

func (testSchema) Classify(c *Candidate, walked bool) (Kind, error) {
	if c.DecodeErr != nil {
		if walked {
			return "", nil
		}
		return "", c.DecodeErr
	}
	if walked && c.Markdown && !c.HasFrontmatter {
		return "", nil
	}
	if s, ok := c.Fields["type"].(string); ok && (s == "widget" || s == "gadget") {
		return Kind(s), nil
	}
	switch filepath.Base(filepath.Dir(c.Path)) {
	case "widgets":
		return "widget", nil
	case "gadgets":
		return "gadget", nil
	}
	if walked {
		return "", nil
	}
	if c.Markdown {
		return "gadget", nil
	}
	return "", fmt.Errorf("%s: cannot tell what kind of resource this is", c.Path)
}

func (testSchema) IsResourceDir(string) (Kind, bool) { return "", false }

func (testSchema) LoadDir(string) (Kind, map[string]any, Payload, error) {
	return "", nil, nil, fmt.Errorf("test schema has no directory resources")
}

func (testSchema) ResourceDirOf(string) (string, bool) { return "", false }

func (testSchema) LockfileName() string { return testLockfileName }

const testLockfileName = "gizmo-lock.json"

func testRegistry() *Registry {
	return NewRegistry(testSchema{},
		KindSpec{
			Kind: "widget", IDPrefix: "wdgt",
			Destroy: Destroy{Verb: "delete", Past: "deleted"},
			Fields: Fields{
				"labels": {Metadata: true},
			},
			Build: buildNamed,
		},
		KindSpec{
			Kind: "gadget", IDPrefix: "gdgt",
			VersionField: "version", VersionIsInt: true,
			Destroy: Destroy{Verb: "archive", Past: "archived"},
			Fields: Fields{
				"widgets": {Ref: &Ref{To: []Kind{"widget"}, List: true, As: EncodeID}},
				"parts":   {Include: true},
			},
			Build: buildNamed,
		},
	)
}

// buildNamed builds a body from a file's fields, defaulting `name` to the
// file's name.
func buildNamed(c *Candidate) (map[string]any, error) {
	if _, ok := c.Fields["name"]; !ok {
		c.Fields["name"] = c.Name
	}
	return c.Fields, nil
}
