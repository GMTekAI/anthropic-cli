package acctest

import (
	"context"

	"github.com/anthropics/anthropic-cli/internal/declarative/core"
)

// recordingClient wraps a Client and remembers every write it was asked to
// make, in order. Reads are not recorded: how many GETs a plan costs is not
// part of the contract, but "a converged apply makes zero writes" is.
type recordingClient struct {
	core.Client
	writes []string
	// created lists every resource a Create returned an ID for, for teardown
	// checks.
	created []createdResource
}

type createdResource struct {
	kind core.Kind
	id   string
}

// Writes returns the writes made since the last ResetWrites, as "create agent",
// "update skill", "destroy environment".
func (r *recordingClient) Writes() []string { return append([]string(nil), r.writes...) }

// ResetWrites forgets the recorded writes. What was created is kept for
// teardown.
func (r *recordingClient) ResetWrites() { r.writes = nil }

// Create keeps the ID from a response that came back with an error, so a
// create that half-failed is still checked at teardown.
func (r *recordingClient) Create(ctx context.Context, kind core.Kind, req core.Request) (map[string]any, error) {
	r.writes = append(r.writes, "create "+string(kind))
	obj, err := r.Client.Create(ctx, kind, req)
	if id, _ := obj["id"].(string); id != "" {
		r.created = append(r.created, createdResource{kind: kind, id: id})
	}
	return obj, err
}

func (r *recordingClient) Update(ctx context.Context, kind core.Kind, id string, req core.Request) (map[string]any, error) {
	r.writes = append(r.writes, "update "+string(kind))
	return r.Client.Update(ctx, kind, id, req)
}

func (r *recordingClient) Destroy(ctx context.Context, kind core.Kind, id string) error {
	r.writes = append(r.writes, "destroy "+string(kind))
	return r.Client.Destroy(ctx, kind, id)
}
