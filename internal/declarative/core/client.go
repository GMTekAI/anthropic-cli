package core

import (
	"context"
	"errors"
)

// ErrNotFound is what a Client returns when a recorded resource is gone from
// the server — deleted out of band, or belonging to a different account than
// the credentials in use.
var ErrNotFound = errors.New("resource not found")

// Payload is content that is not a JSON body — a bundle of files uploaded as
// multipart, for instance. Core never inspects it; it only fingerprints it for
// change detection and hands it back to the Client.
type Payload interface {
	// Fingerprint identifies the payload's content. Two payloads with the same
	// fingerprint are treated as unchanged.
	Fingerprint() (string, error)
	// Describe is a short human summary for plan output, e.g. "3 files".
	Describe() string
}

// Request is what the Client is asked to send: a JSON body, a payload, or both.
type Request struct {
	Body    map[string]any
	Payload Payload
}

// Client is the API surface the reconciler needs. It is an interface so the
// planner and applier can be exercised against a fake without a network.
type Client interface {
	// Get returns the resource as decoded JSON, or an error wrapping
	// ErrNotFound when the server has no such resource.
	Get(ctx context.Context, kind Kind, id string) (map[string]any, error)
	// Create makes a resource and returns the server's answer, which must
	// carry the new resource's string "id". For a payload-backed kind the
	// answer may instead be the version just uploaded, with its identifier
	// under "version".
	Create(ctx context.Context, kind Kind, req Request) (map[string]any, error)
	// Update writes to an existing resource and returns the server's answer,
	// in the same shape Create returns.
	Update(ctx context.Context, kind Kind, id string, req Request) (map[string]any, error)
	// Destroy archives or deletes, per the kind's capabilities. An error
	// wrapping ErrNotFound means the resource was already gone.
	Destroy(ctx context.Context, kind Kind, id string) error
}

// WithHint attaches advice to an error without changing what it is: callers
// that unwrap to the API error underneath still can, and a renderer that asks
// for the hint gets something to tell the user to do.
func WithHint(err error, hint string) error { return &hinted{err, hint} }

// Hint returns the advice attached to err, if any.
func Hint(err error) string {
	var h *hinted
	if errors.As(err, &h) {
		return h.hint
	}
	return ""
}

type hinted struct {
	error
	hint string
}

func (h *hinted) Unwrap() error { return h.error }
